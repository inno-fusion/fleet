import { screen } from "@testing-library/react";
import React from "react";

import createMockConfig from "__mocks__/configMock";
import createMockUser from "__mocks__/userMock";
import { initialState } from "context/app";
import sessionsAPI from "services/entities/sessions";
import {
  createCustomRenderer,
  createMockLocation,
  createMockRouter,
} from "test/test-utils";
import authToken from "utilities/auth_token";

import LoginPage from "./LoginPage";

describe("LoginPage privacy", () => {
  beforeEach(() => {
    jest.spyOn(authToken, "get").mockReturnValue(null);
    jest.spyOn(sessionsAPI, "ssoSettings").mockResolvedValue({
      settings: { sso_enabled: false },
    });
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it("renders without probing Gravatar or requesting vendor default images", async () => {
    const fetchSpy = jest.spyOn(window, "fetch").mockResolvedValue({
      ok: true,
    } as Response);
    const render = createCustomRenderer({
      withBackendMock: true,
      context: { app: initialState },
    });
    const { container } = render(
      <LoginPage router={createMockRouter()} location={createMockLocation()} />
    );

    expect(await screen.findByRole("button", { name: "Log in" })).toBeVisible();
    expect(sessionsAPI.ssoSettings).toHaveBeenCalledTimes(1);
    expect(fetchSpy).not.toHaveBeenCalled();
    container.querySelectorAll("img").forEach((image) => {
      expect(image.src).not.toMatch(/gravatar\.com|fleetdm\.com/);
    });
  });

  it("preserves the operator-configured SSO identity provider image", async () => {
    jest.mocked(sessionsAPI.ssoSettings).mockResolvedValue({
      settings: {
        sso_enabled: true,
        idp_name: "Company IdP",
        idp_image_url: "https://idp.example.com/company-logo.png",
      },
    });
    const render = createCustomRenderer({
      withBackendMock: true,
      context: { app: initialState },
    });
    render(
      <LoginPage router={createMockRouter()} location={createMockLocation()} />
    );
    const ssoButton = await screen.findByRole("button", {
      name: "Sign in with SSO",
    });
    expect(ssoButton.querySelector("img")).toHaveAttribute(
      "src",
      "https://idp.example.com/company-logo.png"
    );
  });

  it("still signs in and updates the local app session", async () => {
    const user = createMockUser();
    jest.spyOn(sessionsAPI, "login").mockResolvedValue({
      user,
      available_teams: [],
      available_fleets: [],
      token: "session-token",
    });
    const saveToken = jest
      .spyOn(authToken, "save")
      .mockImplementation(() => undefined);
    const setCurrentUser = jest.fn();
    const router = createMockRouter();
    const render = createCustomRenderer({
      withBackendMock: true,
      context: {
        app: { ...initialState, config: createMockConfig(), setCurrentUser },
      },
    });
    const { user: browserUser } = render(
      <LoginPage router={router} location={createMockLocation()} />
    );
    await browserUser.type(
      await screen.findByLabelText("Email"),
      "admin@example.com"
    );
    await browserUser.type(screen.getByLabelText("Password"), "password123");
    await browserUser.click(screen.getByRole("button", { name: "Log in" }));
    expect(sessionsAPI.login).toHaveBeenCalledWith({
      email: "admin@example.com",
      password: "password123",
    });
    expect(saveToken).toHaveBeenCalledWith("session-token", undefined);
    expect(setCurrentUser).toHaveBeenCalledWith(user);
    expect(router.push).toHaveBeenCalledWith("/dashboard");
  });
});
