import { screen, waitFor } from "@testing-library/react";
import React, { useContext } from "react";

import { createMockCertificateAuthorityPartial } from "__mocks__/certificatesMock";
import createMockConfig from "__mocks__/configMock";
import createMockUser from "__mocks__/userMock";
import SiteTopNav from "components/top_nav/SiteTopNav";
import AppProvider, { AppContext } from "context/app";
import AuthenticatedRoutes from "router/components/AuthenticatedRoutes";
import configAPI from "services/entities/config";
import usersAPI from "services/entities/users";
import { createCustomRenderer, createMockRouter } from "test/test-utils";
import authToken from "utilities/auth_token";

import fleetLogo from "../../../assets/images/fleet-avatar-24x24@2x.png";

import App, { getEarliestEJBCACertExpiry } from "./App";

const location = { pathname: "/dashboard", search: "", hash: "", query: {} };
const router = createMockRouter();

const AuthenticatedNavigation = () => {
  const { currentUser, config } = useContext(AppContext);
  if (!currentUser || !config) return null;

  return (
    <AuthenticatedRoutes router={router} location={location}>
      <SiteTopNav
        config={config}
        currentUser={currentUser}
        location={location}
        onLogoutUser={jest.fn()}
        onUserMenuItemClick={jest.fn()}
      />
    </AuthenticatedRoutes>
  );
};

describe("App bootstrap privacy", () => {
  const initialize = jest.fn();
  const track = jest.fn();

  beforeEach(() => {
    jest.spyOn(authToken, "get").mockReturnValue("session-token");
    jest.spyOn(window, "scrollTo").mockImplementation(() => undefined);
    Object.defineProperty(window, "pendo", {
      configurable: true,
      value: { initialize, track },
    });
  });

  afterEach(() => {
    Reflect.deleteProperty(window, "pendo");
    jest.restoreAllMocks();
    localStorage.removeItem("fleet-theme");
  });

  it.each(["default", "light", "dark"])(
    "keeps %s organization branding and operational API calls without product tracking",
    async (mode) => {
      localStorage.setItem("fleet-theme", mode === "dark" ? "dark" : "light");
      const config = createMockConfig();
      config.mdm = {
        ...config.mdm,
        enabled_and_configured: false,
        windows_enabled_and_configured: false,
        android_enabled_and_configured: false,
      };
      if (mode !== "default") {
        config.org_info = {
          ...config.org_info,
          org_logo_url_light_mode: "https://images.example.com/logo-light.png",
          org_logo_url_dark_mode: "https://images.example.com/logo-dark.png",
        };
      }
      const user = createMockUser({ gravatar_url: undefined });
      jest.spyOn(usersAPI, "me").mockResolvedValue({
        user,
        available_teams: [],
        settings: { hidden_host_columns: [] },
      });
      jest.spyOn(configAPI, "loadAll").mockResolvedValue(config);
      jest.spyOn(configAPI, "loadEnrollSecret").mockResolvedValue({
        spec: { secrets: [] },
      });
      const render = createCustomRenderer({ withBackendMock: true });
      const { container } = render(
        <AppProvider>
          <App location={location} router={router}>
            <AuthenticatedNavigation />
          </App>
        </AppProvider>
      );

      const logo = await screen.findByAltText("Organization Logo");
      expect(logo).toHaveAttribute(
        "src",
        mode === "default"
          ? fleetLogo
          : `https://images.example.com/logo-${mode}.png`
      );
      expect(
        screen.getByAltText("User avatar").getAttribute("src")
      ).not.toMatch(/gravatar\.com|fleetdm\.com/);
      expect(initialize).not.toHaveBeenCalled();
      expect(track).not.toHaveBeenCalled();
      expect(document.querySelector('script[src*="pendo"]')).toBeNull();
      container.querySelectorAll("img").forEach((image) => {
        expect(image.src).not.toMatch(/gravatar\.com|fleetdm\.com/);
      });
      await waitFor(() => {
        expect(usersAPI.me).toHaveBeenCalled();
        expect(configAPI.loadAll).toHaveBeenCalled();
        expect(configAPI.loadEnrollSecret).toHaveBeenCalled();
      });
    }
  );
});

describe("getEarliestEJBCACertExpiry", () => {
  const ejbca = (mtls_cert_expires_at: string) =>
    createMockCertificateAuthorityPartial({
      type: "ejbca",
      mtls_cert_expires_at,
    });

  it("is the earliest client certificate expiry of the EJBCA certificate authorities, in any order", () => {
    const early = ejbca("2028-02-01T00:00:00Z");
    const late = ejbca("2030-05-01T00:00:00Z");
    // Other types and EJBCA CAs without an expiry don't count.
    const ignored = [
      createMockCertificateAuthorityPartial({
        type: "digicert",
        mtls_cert_expires_at: "2020-01-01T00:00:00Z",
      }),
      createMockCertificateAuthorityPartial({ type: "ejbca" }),
    ];
    expect(getEarliestEJBCACertExpiry([late, early, ...ignored])).toBe(
      "2028-02-01T00:00:00.000Z"
    );
    expect(getEarliestEJBCACertExpiry([early, late, ...ignored])).toBe(
      "2028-02-01T00:00:00.000Z"
    );
  });

  it("is empty when there is no EJBCA certificate authority with an expiry", () => {
    expect(
      getEarliestEJBCACertExpiry([
        createMockCertificateAuthorityPartial({ type: "digicert" }),
      ])
    ).toBe("");
  });
});
