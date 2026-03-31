// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package secrets

import (
	"github.com/gitjet-ru/core-scm/models/db"
	secret_model "github.com/gitjet-ru/core-scm/models/secret"
	"github.com/gitjet-ru/core-scm/modules/log"
	"github.com/gitjet-ru/core-scm/modules/util"
	"github.com/gitjet-ru/core-scm/modules/web"
	"github.com/gitjet-ru/core-scm/services/context"
	"github.com/gitjet-ru/core-scm/services/forms"
	secret_service "github.com/gitjet-ru/core-scm/services/secrets"
)

func SetSecretsContext(ctx *context.Context, ownerID, repoID int64) {
	secrets, err := db.Find[secret_model.Secret](ctx, secret_model.FindSecretsOptions{OwnerID: ownerID, RepoID: repoID})
	if err != nil {
		ctx.ServerError("FindSecrets", err)
		return
	}

	ctx.Data["Secrets"] = secrets
	ctx.Data["DataMaxLength"] = secret_model.SecretDataMaxLength
	ctx.Data["DescriptionMaxLength"] = secret_model.SecretDescriptionMaxLength
}

func PerformSecretsPost(ctx *context.Context, ownerID, repoID int64, redirectURL string) {
	form := web.GetForm(ctx).(*forms.AddSecretForm)

	s, _, err := secret_service.CreateOrUpdateSecret(ctx, ownerID, repoID, form.Name, util.ReserveLineBreakForTextarea(form.Data), form.Description)
	if err != nil {
		log.Error("CreateOrUpdateSecret failed: %v", err)
		ctx.JSONError(ctx.Tr("secrets.save_failed"))
		return
	}

	ctx.Flash.Success(ctx.Tr("secrets.save_success", s.Name))
	ctx.JSONRedirect(redirectURL)
}

func PerformSecretsDelete(ctx *context.Context, ownerID, repoID int64, redirectURL string) {
	id := ctx.FormInt64("id")

	err := secret_service.DeleteSecretByID(ctx, ownerID, repoID, id)
	if err != nil {
		log.Error("DeleteSecretByID(%d) failed: %v", id, err)
		ctx.JSONError(ctx.Tr("secrets.deletion.failed"))
		return
	}

	ctx.Flash.Success(ctx.Tr("secrets.deletion.success"))
	ctx.JSONRedirect(redirectURL)
}
