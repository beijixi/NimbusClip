package auth

import (
	"net/http"
)

const (
	headerUserID   = "X-User-Id"
	headerDeviceID = "X-Device-Id"
)

// Middleware validates basic headers and attaches principal information to context.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get(headerUserID)
		deviceID := r.Header.Get(headerDeviceID)
		if userID == "" || deviceID == "" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"missing authentication headers"}`))
			return
		}
		ctx := WithPrincipal(r.Context(), Principal{UserID: userID, DeviceID: deviceID})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
