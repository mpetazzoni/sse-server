package main

import (
	"net/http"
)

type AuthValidator func(request *http.Request) bool

func NewTokenAuthValidator(token string) AuthValidator {
	return func(request *http.Request) bool {
		return request.Header.Get("Authorization") == "Bearer "+token
	}
}

func NewAllowAllAuthValidator() AuthValidator {
	return func(request *http.Request) bool {
		return true
	}
}
