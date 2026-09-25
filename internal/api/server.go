// Package api 把反渗透核算内核通过 HTTP 对外暴露。
//
// 仅提供 JSON 接口，不做页面。路由：
//
//	GET  /healthz
//	GET  /cases
//	GET  /cases/{name}
//	POST /cases                      登记一份具名工况档
//	POST /calculate                  临时拼一份工况档直接核算
//	POST /cases/{name}/calculate     点名登记表中的工况档核算
package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"rocalc/internal/casefile"
	"rocalc/internal/membrane"
	"rocalc/internal/validate"
)

// Server 持有一个工况档登记表，可在同进程内通过不同实例隔离。
type Server struct {
	registry *casefile.Registry
	mux      *http.ServeMux
}

// New 基于已有登记表创建服务。
func New(r *casefile.Registry) *Server {
	if r == nil {
		r = casefile.DefaultRegistry()
	}
	s := &Server{registry: r, mux: http.NewServeMux()}
	s.routes()
	return s
}

// Handler 返回可挂载的 http.Handler。
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.health)
	s.mux.HandleFunc("GET /cases", s.listCases)
	s.mux.HandleFunc("GET /cases/{name}", s.getCase)
	s.mux.HandleFunc("POST /cases", s.registerCase)
	s.mux.HandleFunc("POST /calculate", s.calculateAdhoc)
	s.mux.HandleFunc("POST /cases/{name}/calculate", s.calculateNamed)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listCases(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"cases": s.registry.Names()})
}

func (s *Server) getCase(w http.ResponseWriter, r *http.Request) {
	c, err := s.registry.Get(r.PathValue("name"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) registerCase(w http.ResponseWriter, r *http.Request) {
	var c casefile.CaseFile
	if err := decodeJSON(r, &c); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	if err := s.registry.Register(c); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) calculateAdhoc(w http.ResponseWriter, r *http.Request) {
	var c casefile.CaseFile
	if err := decodeJSON(r, &c); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	result, err := membrane.Evaluate(c)
	if err != nil {
		writeCalcError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) calculateNamed(w http.ResponseWriter, r *http.Request) {
	c, err := s.registry.Get(r.PathValue("name"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	result, err := membrane.Evaluate(c)
	if err != nil {
		writeCalcError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// writeCalcError 把输入/过程错误映射为 422 并带原因返回。
func writeCalcError(w http.ResponseWriter, err error) {
	var ie *validate.InputError
	if errors.As(err, &ie) {
		writeError(w, http.StatusUnprocessableEntity, "invalid_input", ie.Reasons)
		return
	}
	var de *membrane.DomainError
	if errors.As(err, &de) {
		writeError(w, http.StatusUnprocessableEntity, "invalid_operating_point", de.Error())
		return
	}
	writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, casefile.ErrNotFound):
		writeError(w, http.StatusNotFound, "case_not_found", err.Error())
	case errors.Is(err, casefile.ErrExists):
		writeError(w, http.StatusConflict, "case_exists", err.Error())
	case errors.Is(err, casefile.ErrEmptyName):
		writeError(w, http.StatusBadRequest, "case_name_required", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
	}
}

type errorBody struct {
	Code    string `json:"code"`
	Message any    `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code string, msg any) {
	writeJSON(w, status, errorBody{Code: code, Message: msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	return nil
}
