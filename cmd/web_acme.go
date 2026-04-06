// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gitjet-ru/core-scm/modules/setting"
)

func runACME(listenAddr string, m http.Handler) error {
	_ = listenAddr
	_ = m
	return fmt.Errorf("ACME mode is disabled in this build; use configured TLS certificates instead")
}

func runLetsEncryptFallbackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Use HTTPS", http.StatusBadRequest)
		return
	}
	// Remove the trailing slash at the end of setting.AppURL, the request
	// URI always contains a leading slash, which would result in a double
	// slash
	target := strings.TrimSuffix(setting.AppURL, "/") + r.URL.RequestURI()
	http.Redirect(w, r, target, http.StatusTemporaryRedirect)
}
