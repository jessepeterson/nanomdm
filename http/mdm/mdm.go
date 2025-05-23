package mdm

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	mdmhttp "github.com/micromdm/nanomdm/http"
	"github.com/micromdm/nanomdm/mdm"
	"github.com/micromdm/nanomdm/service"

	"github.com/micromdm/nanolib/log"
	"github.com/micromdm/nanolib/log/ctxlog"
)

// RequestFromHTTP adapts r to an MDM request.
func RequestFromHTTP(r *http.Request) *mdm.Request {
	m := mdm.NewRequestWithContext(r.Context(), GetCert(r.Context()))
	if q := r.URL.Query(); len(q) > 0 {
		m.Params = make(map[string]string, len(q))
		for k, v := range q {
			m.Params[k] = v[0]
		}
	}
	return m
}

// CheckinHandler decodes an MDM check-in request and adapts it to service.
func CheckinHandler(svc service.Checkin, logger log.Logger) http.HandlerFunc {
	if svc == nil {
		panic("nil service")
	}
	if logger == nil {
		panic("nil logger")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		logger := ctxlog.Logger(r.Context(), logger)
		bodyBytes, err := mdmhttp.ReadAllAndReplaceBody(r)
		if err != nil {
			logger.Info("msg", "reading body", "err", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		respBytes, err := service.CheckinRequest(svc, RequestFromHTTP(r), bodyBytes)
		if err != nil {
			logs := []interface{}{"msg", "check-in request"}
			httpStatus := http.StatusInternalServerError
			var statusErr *service.HTTPStatusError
			if errors.As(err, &statusErr) {
				httpStatus = statusErr.Status
				err = fmt.Errorf("HTTP error: %w", statusErr.Unwrap())
			}
			// manualy unwrapping the `StatusErr` is not necessary as `errors.As` manually unwraps
			var parseErr *mdm.ParseError
			if errors.As(err, &parseErr) {
				logs = append(logs, "content", string(parseErr.Content))
				err = fmt.Errorf("parse error: %w", parseErr.Unwrap())
			}
			logs = append(logs, "http_status", httpStatus, "err", err)
			logger.Info(logs...)
			http.Error(w, http.StatusText(httpStatus), httpStatus)
		}
		w.Write(respBytes)
	}
}

// CommandAndReportResultsHandler decodes an MDM command request and adapts it to service.
func CommandAndReportResultsHandler(svc service.CommandAndReportResults, logger log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := ctxlog.Logger(r.Context(), logger)
		bodyBytes, err := mdmhttp.ReadAllAndReplaceBody(r)
		if err != nil {
			logger.Info("msg", "reading body", "err", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		respBytes, err := service.CommandAndReportResultsRequest(svc, RequestFromHTTP(r), bodyBytes)
		if err != nil {
			logs := []interface{}{"msg", "command report results"}
			httpStatus := http.StatusInternalServerError
			var statusErr *service.HTTPStatusError
			if errors.As(err, &statusErr) {
				httpStatus = statusErr.Status
				err = fmt.Errorf("HTTP error: %w", statusErr.Unwrap())
			}
			var parseErr *mdm.ParseError
			if errors.As(err, &parseErr) {
				logs = append(logs, "content", string(parseErr.Content))
				err = fmt.Errorf("parse error: %w", parseErr.Unwrap())
			}
			logs = append(logs, "http_status", httpStatus, "err", err)
			logger.Info(logs...)
			http.Error(w, http.StatusText(httpStatus), httpStatus)
		}
		w.Write(respBytes)
	}
}

// CheckinAndCommandHandler handles both check-in and command requests.
func CheckinAndCommandHandler(service service.CheckinAndCommandService, logger log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		contentType := r.Header.Get("Content-Type")
		if strings.HasPrefix(contentType, "application/x-apple-aspen-mdm-checkin") {
			CheckinHandler(service, logger).ServeHTTP(w, r)
			return
		}
		// assume a non-check-in is a command request
		CommandAndReportResultsHandler(service, logger).ServeHTTP(w, r)
	}
}
