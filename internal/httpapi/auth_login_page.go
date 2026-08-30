package httpapi

import (
	"html/template"
	"net/http"
	"strings"
)

const loginPageTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Sign in · Majesta One</title>
  <style>
    :root {
      --bg0: #0b0f14;
      --bg1: #121820;
      --fg: #e8eef6;
      --muted: #8b9bb0;
      --accent: #3d8bfd;
      --line: rgba(255,255,255,0.08);
    }
    * { box-sizing: border-box; }
    body {
      margin: 0; min-height: 100vh; font-family: "IBM Plex Sans", "Segoe UI", sans-serif;
      color: var(--fg);
      background:
        radial-gradient(1200px 600px at 10% -10%, rgba(61,139,253,0.22), transparent 55%),
        radial-gradient(900px 500px at 100% 0%, rgba(80,200,160,0.12), transparent 50%),
        linear-gradient(165deg, var(--bg0), var(--bg1));
      display: grid; place-items: center; padding: 2rem;
    }
    main {
      width: min(100%, 420px);
      padding: 2.25rem 2rem 1.75rem;
      border: 1px solid var(--line);
      border-radius: 18px;
      background: rgba(12, 16, 22, 0.72);
      backdrop-filter: blur(10px);
      box-shadow: 0 24px 60px rgba(0,0,0,0.35);
    }
    .brand {
      font-family: "IBM Plex Serif", Georgia, serif;
      font-size: 2rem; letter-spacing: -0.03em; margin: 0 0 0.35rem;
    }
    .lede { margin: 0 0 1.75rem; color: var(--muted); line-height: 1.45; font-size: 0.95rem; }
    .actions { display: grid; gap: 0.75rem; }
    a.btn, button.btn {
      display: block; text-align: center; text-decoration: none; color: var(--fg);
      padding: 0.85rem 1rem; border-radius: 12px; border: 1px solid var(--line);
      background: rgba(255,255,255,0.03); font-weight: 600; transition: background .15s, border-color .15s;
      width: 100%; cursor: pointer; font: inherit;
    }
    a.btn:hover, button.btn:hover { background: rgba(61,139,253,0.16); border-color: rgba(61,139,253,0.45); }
    a.btn.primary, button.btn.primary { background: var(--accent); border-color: transparent; color: #061018; }
    a.btn.primary:hover, button.btn.primary:hover { filter: brightness(1.06); background: var(--accent); }
    .hint { margin: 1.25rem 0 0; color: var(--muted); font-size: 0.8rem; line-height: 1.4; }
    .err { margin: 0 0 1rem; padding: 0.75rem 0.9rem; border-radius: 10px;
      background: rgba(220,70,70,0.12); border: 1px solid rgba(220,70,70,0.35); color: #ffb4b4; font-size: 0.9rem; }
    label { display: block; font-size: 0.8rem; color: var(--muted); margin: 0.5rem 0 0.25rem; }
    input {
      width: 100%; padding: 0.7rem 0.8rem; border-radius: 10px; border: 1px solid var(--line);
      background: rgba(0,0,0,0.25); color: var(--fg); font: inherit;
    }
    .divider { margin: 1rem 0; text-align: center; color: var(--muted); font-size: 0.75rem; }
  </style>
</head>
<body>
  <main>
    <h1 class="brand">Majesta One</h1>
    <p class="lede">{{.Lede}}</p>
    {{if .Error}}<p class="err">{{.Error}}</p>{{end}}
    {{if .ShowClaim}}
    <form class="actions" method="post" action="/auth/v1/install/claim">
      <input type="hidden" name="format" value="redirect" />
      <label for="claim_token">Install claim token</label>
      <input id="claim_token" name="token" type="password" autocomplete="off" required />
      <label for="claim_email">Admin email</label>
      <input id="claim_email" name="email" type="email" autocomplete="username" required />
      <label for="claim_password">Password (min 10)</label>
      <input id="claim_password" name="password" type="password" autocomplete="new-password" minlength="10" required />
      <label for="claim_name">Display name (optional)</label>
      <input id="claim_name" name="displayName" type="text" autocomplete="name" />
      <button class="btn primary" type="submit">Claim install</button>
    </form>
    <p class="hint">Day-0 claim creates the first SystemAdmin. Prefer <code>POST /auth/v1/install/claim</code> from curl when not using a browser.</p>
    {{else}}
    <div class="actions">
      {{range .Buttons}}
      <a class="btn {{if .Primary}}primary{{end}}" href="{{.Href}}">{{.Label}}</a>
      {{end}}
    </div>
    {{if .ShowPassword}}
    {{if .Buttons}}<p class="divider">or email &amp; password</p>{{end}}
    <form class="actions" method="post" action="/auth/v1/token">
      <input type="hidden" name="grant_type" value="password" />
      {{if .ClientID}}<input type="hidden" name="client_id" value="{{.ClientID}}" />{{end}}
      {{if .Scope}}<input type="hidden" name="scope" value="{{.Scope}}" />{{end}}
      <label for="pw_email">Email</label>
      <input id="pw_email" name="username" type="email" autocomplete="username" required />
      <label for="pw_password">Password</label>
      <input id="pw_password" name="password" type="password" autocomplete="current-password" required />
      <button class="btn {{if not .Buttons}}primary{{end}}" type="submit">Sign in with password</button>
    </form>
    {{end}}
    {{if and (not .Buttons) (not .ShowPassword)}}
    <p class="err">No sign-in methods are enabled. Configure SSO under Metadata install auth, enable password login, or set social providers.</p>
    {{end}}
    {{end}}
    {{if .Hint}}<p class="hint">{{.Hint}}</p>{{end}}
  </main>
</body>
</html>`

type loginButton struct {
	Label   string
	Href    string
	Primary bool
}

type loginPageData struct {
	Error        string
	Hint         string
	Lede         string
	Buttons      []loginButton
	ShowClaim    bool
	ShowPassword bool
	ClientID     string
	Scope        string
}

func (s *Server) handleAuthLoginPage(w http.ResponseWriter, r *http.Request) {
	clientID := strings.TrimSpace(r.URL.Query().Get("client_id"))
	redirectURI := strings.TrimSpace(r.URL.Query().Get("redirect_uri"))
	codeChallenge := strings.TrimSpace(r.URL.Query().Get("code_challenge"))
	method := strings.TrimSpace(r.URL.Query().Get("code_challenge_method"))
	state := r.URL.Query().Get("state")
	scope := strings.TrimSpace(r.URL.Query().Get("scope"))
	if method == "" {
		method = "S256"
	}

	data := loginPageData{
		Lede:     "Sign in to continue to this Majesta One install.",
		ClientID: clientID,
		Scope:    scope,
	}

	st, err := s.loadInstallAuth(r)
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, "AUTH_POLICY_UNAVAILABLE", "authentication policy unavailable")
		return
	}

	if st != nil && st.ClaimedAt == nil {
		data.ShowClaim = true
		data.Lede = "Claim this install to create the first SystemAdmin (email + password)."
		data.Hint = "You need the INSTALL_CLAIM_TOKEN from deploy secrets."
		s.renderLoginPage(w, data)
		return
	}

	if clientID == "" || redirectURI == "" || codeChallenge == "" {
		// Still allow password form without PKCE params (API / curl users land on token endpoint).
		if st != nil {
			data.ShowPassword = st.PasswordLoginEnabled
		} else {
			data.ShowPassword = true
		}
		if st != nil && st.PublicStatus().SSOConfigured {
			data.Hint = "Open Sign in from Control IDE for SSO (PKCE), or use password / token exchange."
		} else {
			data.Error = "Missing client_id, redirect_uri, or code_challenge for SSO. Password login may still work below."
		}
	}

	envProviders := []string{}
	if s.cfg != nil {
		envProviders = s.cfg.AuthLoginProviders
	}
	var social []string
	s.ensureCustomerOIDCOnBroker(st)

	mk := func(provider, label string, primary bool) loginButton {
		q := r.URL.Query()
		q.Set("provider", provider)
		q.Set("client_id", clientID)
		q.Set("redirect_uri", redirectURI)
		q.Set("code_challenge", codeChallenge)
		q.Set("code_challenge_method", method)
		if state != "" {
			q.Set("state", state)
		}
		if scope != "" {
			q.Set("scope", scope)
		}
		return loginButton{
			Label:   label,
			Href:    "/auth/v1/authorize?" + q.Encode(),
			Primary: primary,
		}
	}

	if st != nil {
		social = st.EffectiveSocialProviders(envProviders)
		data.ShowPassword = st.PasswordLoginEnabled
		if clientID != "" && redirectURI != "" && codeChallenge != "" && s.loginProviderAllowed(st, "oidc") {
			label := st.PublicStatus().IdPDisplayName
			if label == "" {
				label = "SSO"
			}
			data.Buttons = append(data.Buttons, mk("oidc", "Continue with "+label, true))
			data.Hint = "Customer SSO is configured for this install."
		}
	} else {
		social = envProviders
		data.ShowPassword = true
	}

	hasPrimary := len(data.Buttons) > 0
	for _, n := range social {
		if s.loginBroker != nil {
			if _, ok := s.loginBroker.Get(n); !ok {
				continue
			}
		} else {
			continue
		}
		switch n {
		case "google":
			data.Buttons = append(data.Buttons, mk("google", "Continue with Google", !hasPrimary))
			hasPrimary = true
		case "apple":
			data.Buttons = append(data.Buttons, mk("apple", "Continue with Apple", !hasPrimary))
			hasPrimary = true
		case "slack":
			data.Buttons = append(data.Buttons, mk("slack", "Continue with Slack", !hasPrimary))
			hasPrimary = true
		case "dev":
			label := "Continue as local developer"
			if !hasPrimary {
				label = "Continue with local login"
			}
			data.Buttons = append(data.Buttons, mk("dev", label, !hasPrimary))
			hasPrimary = true
			if data.Hint == "" {
				data.Hint = "Local development: built-in Majesta One login provider (no Google Cloud credentials required)."
			}
		}
	}

	s.renderLoginPage(w, data)
}

func (s *Server) renderLoginPage(w http.ResponseWriter, data loginPageData) {
	tmpl, err := template.New("login").Parse(loginPageTmpl)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "TEMPLATE_ERROR", "login page unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
	w.WriteHeader(http.StatusOK)
	_ = tmpl.Execute(w, data)
}
