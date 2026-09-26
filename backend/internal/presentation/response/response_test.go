package response_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-clean-template/backend/internal/kernel/apperr"
	"go-clean-template/backend/internal/presentation/response"

	"github.com/gin-gonic/gin"
)

// errBody 是错误响应的显式解码目标（不用 map[string]string 掩盖结构）。
type errBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// postJSON 对测试路由发起 POST，返回状态码与解码后的错误响应体；解码失败直接 Fatal。
func postJSON(t *testing.T, r *gin.Engine, path, body string) (int, errBody) {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte(body))))
	var b errBody
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
		t.Fatalf("响应体解码失败: %v（原始 %s）", err, w.Body.String())
	}
	return w.Code, b
}

// failRoute 注册一个直接 Fail 指定错误的测试路由，镜像裸签名 handler 的错误路径。
func failRoute(r *gin.Engine, path string, err error) {
	r.POST(path, func(c *gin.Context) { response.Fail(c, err) })
}

// TestFailContract 校验错误翻译契约：表内命中（状态码+文案）、未知错误降级且不泄漏内部细节。
func TestFailContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	failRoute(r, "/login", apperr.ErrCode("bad_credentials").With("no user"))
	failRoute(r, "/register", apperr.ErrCode("email_taken").With("dup"))
	failRoute(r, "/boom", errors.New("db conn refused"))
	failRoute(r, "/timeout", apperr.ErrCode("operation_timeout").With("slow"))
	failRoute(r, "/unavail", apperr.ErrCode("service_unavailable").With("down"))

	if code, b := postJSON(t, r, "/login", "{}"); code != 401 || b.Code != "bad_credentials" || b.Message != "邮箱或密码错误" {
		t.Errorf("bad_credentials 应 401+表内文案，得到 %d %+v", code, b)
	}
	if code, b := postJSON(t, r, "/register", "{}"); code != 409 || b.Code != "email_taken" || b.Message != "邮箱已被注册" {
		t.Errorf("email_taken 应 409+表内文案，得到 %d %+v", code, b)
	}
	if code, b := postJSON(t, r, "/timeout", "{}"); code != 503 || b.Code != "operation_timeout" {
		t.Errorf("operation_timeout 应 503，得到 %d %+v", code, b)
	}
	if code, b := postJSON(t, r, "/unavail", "{}"); code != 503 || b.Code != "service_unavailable" {
		t.Errorf("service_unavailable 应 503，得到 %d %+v", code, b)
	}
	code, b := postJSON(t, r, "/boom", "{}")
	if code != 500 || b.Code != "internal_error" || b.Message != "服务器内部错误" {
		t.Errorf("未知错误应降级 internal_error，得到 %d %+v", code, b)
	}
	if s := wBody(r, "/boom"); bytes.Contains([]byte(s), []byte("db conn")) {
		t.Errorf("500 响应不应泄漏内部错误细节：%s", s)
	}
}

// wBody 返回原始响应体字符串，供泄漏断言。
func wBody(r *gin.Engine, path string) string {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte("{}"))))
	return w.Body.String()
}

// TestWithPreservesChain 校验 apperr.With 保留错误链（errors.Is 穿透）、
// 未入翻译表的码（user_not_found/request_canceled）降级 internal_error——
// 这是内部事实码从不经 handler 合码直接泄漏时的安全兜底。
func TestWithPreservesChain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	failRoute(r, "/nf", apperr.ErrUserNotFound.With("find user", apperr.WithCause(errors.New("driver: bad conn"))))
	failRoute(r, "/cancel", apperr.ErrRequestCanceled.With("query", apperr.WithCause(errors.New("ctx canceled"))))

	wrapped := apperr.ErrDatabase.With("create user", apperr.WithCause(errors.New("driver: bad conn")))
	if !errors.Is(wrapped, apperr.ErrDatabase) {
		t.Error("With 应保留 errors.Is 穿透能力")
	}
	if code, b := postJSON(t, r, "/nf", "{}"); code != 500 || b.Code != "internal_error" {
		t.Errorf("user_not_found 泄漏应降级 internal_error，得到 %d %+v", code, b)
	}
	if code, b := postJSON(t, r, "/cancel", "{}"); code != 500 || b.Code != "internal_error" {
		t.Errorf("request_canceled 泄漏应降级 internal_error，得到 %d %+v", code, b)
	}
	// database_error 已入表：公开该码但文案不含 cause 细节。
	failRoute(r, "/db", wrapped)
	if code, b := postJSON(t, r, "/db", "{}"); code != 500 || b.Code != "database_error" {
		t.Errorf("database_error 应原码 500，得到 %d %+v", code, b)
	}
}

// TestOuterSemanticsWin 校验外层 internal_error 包裹内层公开码时仍 500，
// 不能向深处找到 email_taken 后返回 409。
func TestOuterSemanticsWin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	inner := apperr.ErrCode("email_taken").With("auth.register.email_conflict")
	failRoute(r, "/outer", apperr.ErrCode("internal_error").With("auth.register.unclassified", apperr.WithCause(inner)))

	if code, b := postJSON(t, r, "/outer", "{}"); code != 500 || b.Code != "internal_error" {
		t.Errorf("外层语义应取胜返回 500，得到 %d %+v", code, b)
	}
}

// TestFailNilAndWritten 校验 Fail 守卫：nil 不写响应；已写响应后 Fail 不追加第二份 JSON。
func TestFailNilAndWritten(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/nil", func(c *gin.Context) { response.Fail(c, nil) })
	r.POST("/written", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		response.Fail(c, errors.New("late failure"))
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/nil", bytes.NewReader([]byte("{}"))))
	if w.Code != 200 || w.Body.String() != "" { // Fail(nil) 后无 handler 写响应，ProcessLog 兜底不在本测试链路
		t.Errorf("Fail(nil) 不应写响应，得到 %d %q", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/written", bytes.NewReader([]byte("{}"))))
	if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte(`"ok":true`)) {
		t.Errorf("已写响应不应被追加错误 JSON，得到 %d %s", w.Code, w.Body.String())
	}
}

// TestAckContract 校验无数据负载端点的统一执行成功响应：200 + 精确 {"status":"ok"}。
func TestAckContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/logout", response.Ack)
	r.GET("/healthz", response.Ack)

	for _, m := range []string{http.MethodPost, http.MethodGet} {
		path := "/logout"
		if m == http.MethodGet {
			path = "/healthz"
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(m, path, nil))
		if w.Code != 200 || w.Body.String() != `{"status":"ok"}` {
			t.Errorf("Ack 应渲染 200 {\"status\":\"ok\"}，得到 %d %s", w.Code, w.Body.String())
		}
	}
}

// bindReq 是绑定惯用法测试用的请求体类型；tag 层失败由框架处理。
type bindReq struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Validate 模拟 DTO 自持校验：短密码拒绝。
func (r *bindReq) Validate() error {
	if len(r.Password) < 8 {
		return apperr.ErrInvalidInput.With("password too short")
	}
	return nil
}

// TestBindIdiomContract 校验裸签名 handler 的绑定惯用法：
// ShouldBindJSON 失败经 InvalidBind 归一为 400 invalid_input（不透出字段细节），
// Validate 失败同样 400；合法体到达 handler 并由 response.OK 渲染 200 JSON。
func TestBindIdiomContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var got bindReq
	r.POST("/bind", func(c *gin.Context) {
		var req bindReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, response.InvalidBind(err))
			return
		}
		if err := req.Validate(); err != nil {
			response.Fail(c, err)
			return
		}
		got = req
		response.OK(c, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/bind", bytes.NewReader([]byte(`{"email":"a@b.com","password":"12345678"}`))))
	if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte(`"ok":true`)) {
		t.Errorf("成功路径应 200 JSON，得到 %d %s", w.Code, w.Body.String())
	} else if got.Email != "a@b.com" || got.Password != "12345678" {
		t.Errorf("handler 应收到类型化入参，得到 %+v", got)
	}

	// required 失败（tag 层）：400 invalid_input，无字段细节
	if code, b := postJSON(t, r, "/bind", `{"email":"","password":"1"}`); code != 400 || b.Code != "invalid_input" {
		t.Errorf("校验失败应 400 invalid_input，得到 %d %+v", code, b)
	}
	// required 通过但 Validate 拒绝（短密码）：同样 400 invalid_input
	if code, b := postJSON(t, r, "/bind", `{"email":"a@b.com","password":"123"}`); code != 400 || b.Code != "invalid_input" {
		t.Errorf("Validate 失败应 400 invalid_input，得到 %d %+v", code, b)
	}
	if code, b := postJSON(t, r, "/bind", `not json`); code != 400 || b.Code != "invalid_input" {
		t.Errorf("非 JSON 体应 400 invalid_input，得到 %d %+v", code, b)
	}
}
