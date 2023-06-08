// Copyright 2018 Frédéric Guillot. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package ui // import "miniflux.app/ui"

import (
	"net/http"

	"miniflux.app/config"
	"miniflux.app/http/request"
	"miniflux.app/http/response/html"
	"miniflux.app/http/route"
	"miniflux.app/logger"
	"miniflux.app/model"
	feedHandler "miniflux.app/reader/handler"
	"miniflux.app/reader/subscription"
	"miniflux.app/ui/form"
	"miniflux.app/ui/session"
	"miniflux.app/ui/view"
)

func (h *handler) submitSubscription(w http.ResponseWriter, r *http.Request) {
	sess := session.New(h.store, request.SessionID(r))
	v := view.New(h.tpl, r, sess)

	user, err := h.store.UserByID(request.UserID(r))
	if err != nil {
		html.ServerError(w, r, err)
		return
	}

	categories, err := h.store.Categories(user.ID)
	if err != nil {
		html.ServerError(w, r, err)
		return
	}

	v.Set("categories", categories)
	v.Set("menu", "feeds")
	v.Set("user", user)
	v.Set("countUnread", h.store.CountUnreadEntries(user.ID))
	v.Set("countErrorFeeds", h.store.CountUserFeedsWithErrors(user.ID))
	v.Set("defaultUserAgent", config.Opts.HTTPClientUserAgent())
	v.Set("hasProxyConfigured", config.Opts.HasHTTPClientProxyConfigured())

	subscriptionForm := form.NewSubscriptionForm(r)
	if err := subscriptionForm.Validate(); err != nil {
		v.Set("form", subscriptionForm)
		v.Set("errorMessage", err.Error())
		html.OK(w, r, v.Render("add_subscription"))
		return
	}

	var rssbridgeURL string
	intg, err := h.store.Integration(user.ID)
	if err != nil {
		logger.Error("[UI:SubmitSubscription] Get integrations for user %d failed: %v", user.ID, err)
	} else if intg != nil && intg.RSSBridgeEnabled {
		rssbridgeURL = intg.RSSBridgeURL
	}

	subscriptions, findErr := subscription.FindSubscriptions(
		subscriptionForm.URL,
		subscriptionForm.UserAgent,
		subscriptionForm.Cookie,
		subscriptionForm.Username,
		subscriptionForm.Password,
		subscriptionForm.FetchViaProxy,
		subscriptionForm.AllowSelfSignedCertificates,
		rssbridgeURL,
	)

	if findErr != nil {
		logger.Error("[UI:SubmitSubscription] %q -> %s", subscriptionForm.URL, findErr)
	}

	logger.Debug("[UI:SubmitSubscription] %s", subscriptions)

	n := len(subscriptions)
	switch {
	case n == 0:
		v.Set("form", subscriptionForm)
		if findErr != nil {
			v.Set("errorMessage", findErr)
		} else {
			v.Set("errorMessage", "error.subscription_not_found")
		}
		html.OK(w, r, v.Render("add_subscription"))
	case n == 1 && findErr == nil:
		feed, err := feedHandler.CreateFeed(h.store, user.ID, &model.FeedCreationRequest{
			CategoryID:                  subscriptionForm.CategoryID,
			FeedURL:                     subscriptions[0].URL,
			Crawler:                     subscriptionForm.Crawler,
			AllowSelfSignedCertificates: subscriptionForm.AllowSelfSignedCertificates,
			UserAgent:                   subscriptionForm.UserAgent,
			Cookie:                      subscriptionForm.Cookie,
			Username:                    subscriptionForm.Username,
			Password:                    subscriptionForm.Password,
			ScraperRules:                subscriptionForm.ScraperRules,
			RewriteRules:                subscriptionForm.RewriteRules,
			BlocklistRules:              subscriptionForm.BlocklistRules,
			KeeplistRules:               subscriptionForm.KeeplistRules,
			UrlRewriteRules:             subscriptionForm.UrlRewriteRules,
			FetchViaProxy:               subscriptionForm.FetchViaProxy,
		})
		if err != nil {
			v.Set("form", subscriptionForm)
			v.Set("errorMessage", err)
			html.OK(w, r, v.Render("add_subscription"))
			return
		}

		html.Redirect(w, r, route.Path(h.router, "feedEntries", "feedID", feed.ID))
	default:
		v := view.New(h.tpl, r, sess)
		if findErr != nil {
			v.Set("errorMessage", findErr)
		}
		v.Set("subscriptions", subscriptions)
		v.Set("form", subscriptionForm)
		v.Set("menu", "feeds")
		v.Set("user", user)
		v.Set("countUnread", h.store.CountUnreadEntries(user.ID))
		v.Set("countErrorFeeds", h.store.CountUserFeedsWithErrors(user.ID))
		v.Set("hasProxyConfigured", config.Opts.HasHTTPClientProxyConfigured())

		html.OK(w, r, v.Render("choose_subscription"))
	}
}
