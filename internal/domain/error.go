package domain

import "errors"

// Domain errors - переносим из BusinessThing и адаптируем под наши нужды
var (
	// ErrNotFound - ресурс не найден (404)
	ErrNotFound = errors.New("resource not found")

	// ErrTeamExists - команда уже существует (400)
	ErrTeamExists = errors.New("team already exists")

	// ErrPRExists - PR уже существует (409)
	ErrPRExists = errors.New("pull request already exists")

	// ErrPRMerged - нельзя изменять merged PR (409)
	ErrPRMerged = errors.New("cannot modify merged pull request")

	// ErrNotAssigned - пользователь не назначен ревьювером (409)
	ErrNotAssigned = errors.New("user is not assigned as reviewer")

	// ErrNoCandidate - нет доступных кандидатов для назначения (409)
	ErrNoCandidate = errors.New("no active candidate available for assignment")

	// ErrInvalidArgument - невалидный аргумент (400)
	ErrInvalidArgument = errors.New("invalid argument")
)

type ErrorCode string

const (
	ErrorCodeTeamExists      ErrorCode = "TEAM_EXISTS"
	ErrorCodePRExists        ErrorCode = "PR_EXISTS"
	ErrorCodePRMerged        ErrorCode = "PR_MERGED"
	ErrorCodeNotAssigned     ErrorCode = "NOT_ASSIGNED"
	ErrorCodeNoCandidate     ErrorCode = "NO_CANDIDATE"
	ErrorCodeNotFound        ErrorCode = "NOT_FOUND"
	ErrorCodeInvalidArgument ErrorCode = "INVALID_ARGUMENT"
)

func GetErrorCode(err error) ErrorCode {
	switch {
	case errors.Is(err, ErrTeamExists):
		return ErrorCodeTeamExists
	case errors.Is(err, ErrPRExists):
		return ErrorCodePRExists
	case errors.Is(err, ErrPRMerged):
		return ErrorCodePRMerged
	case errors.Is(err, ErrNotAssigned):
		return ErrorCodeNotAssigned
	case errors.Is(err, ErrNoCandidate):
		return ErrorCodeNoCandidate
	case errors.Is(err, ErrNotFound):
		return ErrorCodeNotFound
	case errors.Is(err, ErrInvalidArgument):
		return ErrorCodeInvalidArgument
	default:
		return ""
	}
}

func GetHTTPStatus(err error) int {
	switch {
	case errors.Is(err, ErrNotFound):
		return 404
	case errors.Is(err, ErrTeamExists):
		return 400
	case errors.Is(err, ErrPRExists), errors.Is(err, ErrPRMerged),
		errors.Is(err, ErrNotAssigned), errors.Is(err, ErrNoCandidate):
		return 409
	case errors.Is(err, ErrInvalidArgument):
		return 400
	default:
		return 500
	}
}
