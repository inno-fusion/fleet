package mail

import (
	"html/template"
	"strings"
	"testing"

	"github.com/fleetdm/fleet/v4/server/fleet"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

func TestTransactionalTemplatesHaveNoRemoteResources(t *testing.T) {
	const baseURL = "https://fleet.example.com/subpath"
	const token = "test-token"
	for _, tc := range []struct {
		name   string
		mailer fleet.Mailer
		link   string
		text   string
	}{
		{"smtp", &SMTPTestMailer{BaseURL: baseURL}, baseURL, "Fleet SMTP setup confirmed"},
		{"password reset", &PasswordResetMailer{BaseURL: baseURL, Token: token}, baseURL + "/login/reset?token=" + token, "Reset password"},
		{"change email", &ChangeEmailMailer{BaseURL: baseURL, Token: token}, baseURL + "/email/change/" + token, "Confirm email"},
		{"mfa", &MFAMailer{BaseURL: baseURL, Token: token, FullName: "<script>Test</script>"}, baseURL + "/login/mfa/" + token, "&lt;script&gt;Test&lt;/script&gt;"},
		{"invite", &InviteMailer{BaseURL: baseURL, Invite: &fleet.Invite{Token: token, Name: "<b>Test</b>"}, OrgName: "Example"}, baseURL + "/login/invites/" + token, "&lt;b&gt;Test&lt;/b&gt;"},
		{"sso invite", &InviteMailer{BaseURL: baseURL, Invite: &fleet.Invite{Token: token, SSOEnabled: true}}, baseURL + "/login/ssoinvites/" + token, "Accept invitation"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := tc.mailer.Message()
			require.NoError(t, err)
			require.Contains(t, string(body), tc.text)
			require.NotContains(t, string(body), "ZgotmplZ")
			require.NotContains(t, string(body), "fonts.googleapis.com")
			require.NotContains(t, string(body), "fonts.gstatic.com")
			require.NotContains(t, string(body), "{{")
			root, err := html.Parse(strings.NewReader(string(body)))
			require.NoError(t, err)
			var links []string
			var visit func(*html.Node)
			visit = func(n *html.Node) {
				if n.Type == html.ElementNode {
					require.NotContains(t, []string{"img", "link", "script", "iframe", "object", "embed"}, n.Data)
					for _, attr := range n.Attr {
						require.NotContains(t, []string{"src", "srcset", "background"}, attr.Key)
						if n.Data == "a" && attr.Key == "href" {
							links = append(links, attr.Val)
						}
					}
				}
				for child := n.FirstChild; child != nil; child = child.NextSibling {
					visit(child)
				}
			}
			visit(root)
			require.Contains(t, links, tc.link)
			require.NotContains(t, strings.ToLower(string(body)), "url(")
			require.NotContains(t, strings.ToLower(string(body)), "@import")
		})
	}
}

func TestLegacyAssetURLDoesNotAddResources(t *testing.T) {
	mailer := &SMTPTestMailer{
		BaseURL:  "https://fleet.example.com",
		AssetURL: template.URL("https://assets.example.com"),
	}
	body, err := mailer.Message()
	require.NoError(t, err)
	require.NotContains(t, string(body), "assets.example.com")
}
