package api

import (
	"fmt"
	"net/http"
	"time"

	jwtService "github.com/madappgang/identifo/jwt/service"
	"github.com/madappgang/identifo/model"
	"github.com/madappgang/identifo/web/middleware"
	"github.com/xlzd/gotp"
)

// EnableTFA enables two-factor authentication for the user.
func (ar *Router) EnableTFA() http.HandlerFunc {
	type tfaSecret struct {
		TFASecret string `json:"tfa_secret"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		app := middleware.AppFromContext(r.Context())
		if app == nil {
			ar.Error(w, ErrorAPIRequestAppIDInvalid, http.StatusBadRequest, "App is not in context.", "EnableTFA.AppFromContext")
			return
		}

		if tfaStatus := app.TFAStatus(); tfaStatus != model.TFAStatusOptional {
			ar.Error(w, ErrorAPIRequestBodyParamsInvalid, http.StatusBadRequest, fmt.Sprintf("App TFA status is '%s', not 'optional'", tfaStatus), "EnableTFA.TFAStatus")
			return
		}

		accessTokenBytes, ok := r.Context().Value(model.TokenRawContextKey).([]byte)
		if !ok {
			ar.Error(w, ErrorAPIRequestAppIDInvalid, http.StatusBadRequest, "Token bytes are not in context.", "EnableTFA.TokenBytesFromContext")
			return
		}

		// Get userID from token and update user with this ID.
		userID, err := ar.getTokenSubject(string(accessTokenBytes))
		if err != nil {
			ar.Error(w, ErrorAPIAppCannotExtractTokenSubject, http.StatusInternalServerError, err.Error(), "EnableTFA.getTokenSubject")
			return
		}

		user, err := ar.userStorage.UserByID(userID)
		if err != nil {
			ar.Error(w, ErrorAPIUserNotFound, http.StatusBadRequest, err.Error(), "EnableTFA.UserByID")
			return
		}

		if tfaInfo := user.TFAInfo(); tfaInfo.IsEnabled && tfaInfo.Secret != "" {
			ar.Error(w, ErrorAPIRequestTFAAlreadyEnabled, http.StatusBadRequest, "TFA already enabled for this user", "EnableTFA.alreadyEnabled")
			return
		}

		tfa := model.TFAInfo{
			IsEnabled: true,
			Secret:    gotp.RandomSecret(16),
		}
		user.SetTFAInfo(tfa)

		if _, err := ar.userStorage.UpdateUser(userID, user); err != nil {
			ar.Error(w, ErrorAPIInternalServerError, http.StatusInternalServerError, err.Error(), "EnableTFA.UpdateUser")
			return
		}

		switch ar.tfaType {
		case model.TFATypeApp:
			ar.ServeJSON(w, http.StatusOK, &tfaSecret{TFASecret: tfa.Secret})
			return
		case model.TFATypeSMS:
			ar.sendTFASecretInSMS(w, tfa.Secret)
			return
		case model.TFATypeEmail:
			ar.sendTFASecretInEmail(w, tfa.Secret)
			return
		}
		ar.Error(w, ErrorAPIInternalServerError, http.StatusInternalServerError, fmt.Sprintf("Unknown tfa type '%s'", ar.tfaType), "switch.tfaType")
	}
}

// FinalizeTFA finalizes two-factor authentication.
func (ar *Router) FinalizeTFA() http.HandlerFunc {
	type requestBody struct {
		TFACode string   `json:"tfa_code"`
		Scopes  []string `json:"scopes"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		d := requestBody{}
		if ar.MustParseJSON(w, r, &d) != nil {
			return
		}

		if len(d.TFACode) == 0 {
			ar.Error(w, ErrorAPIRequestTFACodeEmpty, http.StatusBadRequest, "", "FinalizeTFA.empty")
			return
		}

		oldAccessTokenBytes, ok := r.Context().Value(model.TokenRawContextKey).([]byte)
		if !ok {
			ar.Error(w, ErrorAPIRequestTokenInvalid, http.StatusBadRequest, "Token bytes are not in context.", "FinalizeTFA.TokenBytesFromContext")
			return
		}
		oldAccessTokenString := string(oldAccessTokenBytes)

		userID, err := ar.getTokenSubject(oldAccessTokenString)
		if err != nil {
			ar.Error(w, ErrorAPIAppCannotExtractTokenSubject, http.StatusInternalServerError, err.Error(), "FinalizeTFA.getTokenSubject")
			return
		}

		user, err := ar.userStorage.UserByID(userID)
		if err != nil {
			ar.Error(w, ErrorAPIUserNotFound, http.StatusBadRequest, err.Error(), "FinalizeTFA.UserByID")
			return
		}

		totp := gotp.NewDefaultTOTP(user.TFAInfo().Secret)
		if verified := totp.Verify(d.TFACode, int(time.Now().Unix())); !verified {
			ar.Error(w, ErrorAPIRequestTFACodeInvalid, http.StatusUnauthorized, "", "FinalizeTFA.TOTP_Invalid")
			return
		}

		// Issue new access, and, if requested, refresh token, and then invalidate the old one.
		scopes, err := ar.userStorage.RequestScopes(user.ID(), d.Scopes)
		if err != nil {
			ar.Error(w, ErrorAPIRequestScopesForbidden, http.StatusForbidden, err.Error(), "LoginWithPassword.RequestScopes")
			return
		}

		app := middleware.AppFromContext(r.Context())
		if app == nil {
			ar.Error(w, ErrorAPIRequestAppIDInvalid, http.StatusBadRequest, "App is not in context.", "EnableTFA.AppFromContext")
			return
		}

		offline := contains(scopes, jwtService.OfflineScope)
		accessToken, refreshToken, err := ar.loginUser(user, d.Scopes, app, offline, false)
		if err != nil {
			ar.Error(w, ErrorAPIAppAccessTokenNotCreated, http.StatusInternalServerError, err.Error(), "LoginWithPassword.loginUser")
			return
		}

		// Blacklist old access token.
		if err := ar.tokenBlacklist.Add(oldAccessTokenString); err != nil {
			ar.logger.Printf("Cannot blacklist old access token: %s\n", err)
		}

		result := &AuthResponse{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
		}

		ar.userStorage.UpdateLoginMetadata(user.ID())
		ar.ServeJSON(w, http.StatusOK, result)
	}
}

/* TODO
func (ar *Router) RequestDisabledTFA() http.HandlerFunc {

}
*/

func (ar *Router) sendTFASecretInSMS(w http.ResponseWriter, tfaSecret string) {
	ar.Error(w, ErrorAPIInternalServerError, http.StatusBadRequest, "Not yet implemented", "sendTFASecretInSMS")
}

func (ar *Router) sendTFASecretInEmail(w http.ResponseWriter, tfaSecret string) {
	ar.Error(w, ErrorAPIInternalServerError, http.StatusBadRequest, "Not yet implemented", "sendTFASecretInEmail")
}
