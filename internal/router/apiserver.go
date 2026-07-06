package router

import (
	"net/http"

	apiTypes "github.com/oapi-codegen/runtime/types"
)

func (s *apiServer) RegisterUser(w http.ResponseWriter, r *http.Request) {
	s.auth.RegisterUser(w, r)
}
func (s *apiServer) LoginUser(w http.ResponseWriter, r *http.Request) {
	s.auth.LoginUser(w, r)
}
func (s *apiServer) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	s.auth.GetCurrentUser(w, r)
}
func (s *apiServer) ListApprovedUsers(w http.ResponseWriter, r *http.Request) {
	s.auth.ListApprovedUsers(w, r)
}
func (s *apiServer) CreateApprovedUser(w http.ResponseWriter, r *http.Request) {
	s.auth.CreateApprovedUser(w, r)
}
func (s *apiServer) BulkCreateApprovedUsers(w http.ResponseWriter, r *http.Request) {
	s.auth.BulkCreateApprovedUsers(w, r)
}
func (s *apiServer) DeleteApprovedUser(w http.ResponseWriter, r *http.Request, id apiTypes.UUID) {
	s.auth.DeleteApprovedUser(w, r, id)
}

func (s *apiServer) ListTodos(w http.ResponseWriter, r *http.Request) {
	s.todo.ListTodos(w, r)
}
func (s *apiServer) CreateTodo(w http.ResponseWriter, r *http.Request) {
	s.todo.CreateTodo(w, r)
}
func (s *apiServer) GetTodo(w http.ResponseWriter, r *http.Request, id apiTypes.UUID) {
	s.todo.GetTodo(w, r, id)
}
func (s *apiServer) UpdateTodo(w http.ResponseWriter, r *http.Request, id apiTypes.UUID) {
	s.todo.UpdateTodo(w, r, id)
}
func (s *apiServer) DeleteTodo(w http.ResponseWriter, r *http.Request, id apiTypes.UUID) {
	s.todo.DeleteTodo(w, r, id)
}
