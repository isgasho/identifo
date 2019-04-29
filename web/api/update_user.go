package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/madappgang/identifo/model"
)

// UpdateUser allows to change user login and password.
func (ar *Router) UpdateUser() http.HandlerFunc {

	type updateResponse struct {
		Message string `json:"message"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		d := updateData{}
		if ar.MustParseJSON(w, r, &d) != nil {
			return
		}
		userID := tokenFromContext(r.Context()).UserID()
		user, err := ar.userStorage.UserByID(userID)
		if err != nil {
			ar.Error(w, err, http.StatusUnauthorized, "Not authorized")
			return
		}

		if err := d.validate(user); err != nil {
			ar.Error(w, err, http.StatusBadRequest, err.Error())
			return
		}
		// check that new username is not taken.
		if d.updateUsername && ar.userStorage.UserExists(d.NewUsername) {
			ar.Error(w, errors.New("Username is occupied. "), http.StatusBadRequest, "Username is occupied. Try to choose another one.")
			return
		}

		// check that email is not taken.
		if d.updateEmail {
			if _, err := ar.userStorage.UserByEmail(d.NewEmail); err == nil {
				ar.Error(w, errors.New("Email is occupied. "), http.StatusBadRequest, "Email is occupied. Try to choose another one.")
				return
			}
		}

		// update password
		if d.updatePassword {
			// check old password.
			_, err := ar.userStorage.UserByNamePassword(user.Name(), d.OldPassword)
			if err != nil {
				ar.Error(w, err, http.StatusBadRequest, "Invalid old password.")
				return
			}

			// save new password.
			err = ar.userStorage.ResetPassword(user.ID(), d.NewPassword)
			if err != nil {
				ar.Error(w, err, http.StatusInternalServerError, "Unable to update password. Please try again.")
				return
			}
		}

		// change username if user specified new one.
		if d.updateUsername {
			user.SetName(d.NewUsername)
		}

		if d.updateEmail {
			user.SetEmail(d.NewEmail)
		}

		if d.updateUsername || d.updateEmail {
			_, err = ar.userStorage.UpdateUser(userID, user)
			if err != nil {
				ar.Error(w, err, http.StatusBadRequest, "Unable to update username or email. Please try again.")
				return
			}
		}

		// prepare response
		updatedFields := []string{}
		if d.updateUsername {
			updatedFields = append(updatedFields, "username")
		}
		if d.updateEmail {
			updatedFields = append(updatedFields, "email")
		}
		if d.updatePassword {
			updatedFields = append(updatedFields, "password")
		}

		msg := "Nothing changed."
		if len(updatedFields) > 0 {
			updatedFields[0] = strings.Title(updatedFields[0])
			msg = strings.Join(updatedFields, ", ") + " changed. "
		}
		response := updateResponse{
			Message: msg,
		}
		ar.ServeJSON(w, http.StatusOK, response)
	}

}

type updateData struct {
	NewEmail       string `json:"new_email"`
	NewUsername    string `json:"new_username,omitempty"`
	NewPassword    string `json:"new_password,omitempty"`
	OldPassword    string `json:"old_password,omitempty"`
	updatePassword bool
	updateEmail    bool
	updateUsername bool
}

func (d *updateData) validate(user model.User) error {
	if d.NewUsername != "" && user.Name() != d.NewUsername {
		d.updateUsername = true
	}
	if d.NewEmail != "" && user.Email() != d.NewEmail {
		d.updateEmail = true
	}
	if d.NewPassword != "" && d.NewPassword != d.OldPassword {
		d.updatePassword = true
	}

	if d.updatePassword {
		if d.OldPassword == "" {
			return errors.New("Old password is not specified. ")
		}
		// validate password
		if err := model.StrongPswd(d.NewPassword); err != nil {
			return errors.New("New password is not strong enough. ")
		}
	}

	if d.updateEmail && !emailRegexp.MatchString(d.NewEmail) {
		return errors.New("Email is not valid. ")
	}
	return nil
}