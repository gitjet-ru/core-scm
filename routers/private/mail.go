// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	stdCtx "context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gitjet-ru/core-scm/models/db"
	user_model "github.com/gitjet-ru/core-scm/models/user"
	"github.com/gitjet-ru/core-scm/modules/json"
	"github.com/gitjet-ru/core-scm/modules/log"
	"github.com/gitjet-ru/core-scm/modules/private"
	"github.com/gitjet-ru/core-scm/modules/setting"
	"github.com/gitjet-ru/core-scm/services/context"
	"github.com/gitjet-ru/core-scm/services/mailer"
	sender_service "github.com/gitjet-ru/core-scm/services/mailer/sender"
)

// SendEmail pushes messages to mail queue
//
// It doesn't wait before each message will be processed
func SendEmail(ctx *context.PrivateContext) {
	if setting.MailService == nil {
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: "Mail service is not enabled.",
		})
		return
	}

	var mail private.Email
	rd := ctx.Req.Body
	defer rd.Close()

	if err := json.NewDecoder(rd).Decode(&mail); err != nil {
		log.Error("JSON Decode failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: err.Error(),
		})
		return
	}

	var emails []string
	if len(mail.To) > 0 {
		for _, uname := range mail.To {
			user, err := user_model.GetUserByName(ctx, uname)
			if err != nil {
				err := fmt.Sprintf("Failed to get user information: %v", err)
				log.Error(err)
				ctx.JSON(http.StatusInternalServerError, private.Response{
					Err: err,
				})
				return
			}

			if user != nil && len(user.Email) > 0 {
				emails = append(emails, user.Email)
			}
		}
	} else {
		err := db.Iterate(ctx, nil, func(ctx stdCtx.Context, user *user_model.User) error {
			if len(user.Email) > 0 && user.IsActive {
				emails = append(emails, user.Email)
			}
			return nil
		})
		if err != nil {
			err := fmt.Sprintf("Failed to find users: %v", err)
			log.Error(err)
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: err,
			})
			return
		}
	}

	sendEmail(ctx, mail.Subject, mail.Message, emails)
}

func sendEmail(ctx *context.PrivateContext, subject, message string, to []string) {
	for _, email := range to {
		msg := sender_service.NewMessage(email, subject, message)
		mailer.SendAsync(msg)
	}

	wasSent := strconv.Itoa(len(to))

	ctx.PlainText(http.StatusOK, wasSent)
}
