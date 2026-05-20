package pkg

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

var Validate *validator.Validate

func init() {
	Validate = validator.New()
	Validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		tag := fld.Tag.Get("json")
		name := strings.SplitN(tag, ",", 2)[0]
		if name == "" || name == "-" {
			return fld.Name
		}
		return name
	})
}

type ValidationErrors map[string]string

func ValidateStruct(s interface{}) ValidationErrors {
	err := Validate.Struct(s)
	if err == nil {
		return nil
	}
	verrs, ok := err.(validator.ValidationErrors)
	if !ok {
		return ValidationErrors{"_error": "некорректные данные"}
	}
	errors := make(ValidationErrors)
	for _, e := range verrs {
		field := e.Field()
		if field == "" {
			field = e.StructField()
		}
		errors[field] = tagToRussian(e)
	}
	return errors
}

func tagToRussian(e validator.FieldError) string {
	param := e.Param()
	switch e.Tag() {
	case "required":
		return "обязательное поле"
	case "email":
		return "некорректный формат email"
	case "min":
		return fmt.Sprintf("минимум %s", param)
	case "max":
		return fmt.Sprintf("максимум %s", param)
	case "gt":
		return fmt.Sprintf("должно быть больше %s", param)
	case "gte":
		return fmt.Sprintf("должно быть не менее %s", param)
	case "lt":
		return fmt.Sprintf("должно быть меньше %s", param)
	case "lte":
		return fmt.Sprintf("должно быть не более %s", param)
	case "url":
		return "некорректный URL"
	case "uuid":
		return "недействительный UUID"
	case "datetime":
		return "некорректный формат даты"
	default:
		return fmt.Sprintf("не прошло валидацию: %s", e.Tag())
	}
}

func WriteValidationErrors(w http.ResponseWriter, verrs ValidationErrors) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	resp := map[string]any{"errors": verrs}
	json.NewEncoder(w).Encode(resp)
}
