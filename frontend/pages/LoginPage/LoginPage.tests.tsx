import { screen, waitFor } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import React from "react";
import { QueryClient, QueryClientProvider } from "react-query";

import { createMockConfig } from "__mocks__/configMock";
import createMockUser from "__mocks__/userMock";
import { initialState } from "context/app";
import paths from "router/paths";
import sessionsAPI from "services/entities/sessions";
import mockServer from "test/mock-server";
import {
  baseUrl,
  createCustomRenderer,
  createMockLocation,
  createMockRouter,
} from "test/test-utils";
import authToken from "utilities/auth_token";

import LoginPage from "./LoginPage";

const ssoEnabledHandler = http.get("/api/v1/fleet/sso", () =>
  HttpResponse.json({ settings: { sso_enabled: true } })
);

const gravatarHandler = http.get(
  "https://www.gravatar.com/avatar/0000",
  () => new HttpResponse(null, { status: 404 })
);

const renderLoginPage = (
  query: Record<string, string> = {},
  currentUser = null as ReturnType<typeof createMockUser> | null
) => {
  const render = createCustomRenderer({
    withBackendMock: true,
    context: { app: { currentUser } },
  });
  return render(
    <LoginPage
      router={createMockRouter()}
      location={createMockLocation({ pathname: "/login", query })}
    />
  );
};

describe("LoginPage", () => {
  beforeEach(() => {
    mockServer.use(ssoEnabledHandler, gravatarHandler);
  });

  afterEach(() => {
    authToken.remove();
  });

  it("tells the user their session expired", async () => {
    renderLoginPage({ session_expired: "true" });

    expect(
      await screen.findByText("Your session expired. Log in to continue.")
    ).toBeInTheDocument();
  });

  it("does not show the session-expired notice on a regular visit", async () => {
    renderLoginPage();

    await screen.findByRole("button", { name: "Log in" });
    expect(
      screen.queryByText("Your session expired. Log in to continue.")
    ).not.toBeInTheDocument();
  });

  it("offers SSO when the expired session's user is still in app context", async () => {
    renderLoginPage({ session_expired: "true" }, createMockUser());

    expect(
      await screen.findByRole("button", { name: /sign in with sso/i })
    ).toBeInTheDocument();
  });

  it("drops the previous user's cached data after logging in", async () => {
    mockServer.use(
      http.post(baseUrl("/login"), () =>
        HttpResponse.json({
          user: createMockUser({ id: 2, email: "next@example.com" }),
          available_teams: [],
          token: "next-user-token",
        })
      )
    );
    const queryClient = new QueryClient();
    queryClient.setQueryData([1, 7], { report: "previous user's results" });
    const router = createMockRouter();
    const render = createCustomRenderer({
      context: {
        app: {
          config: createMockConfig(),
          setCurrentUser: jest.fn(),
          setAvailableTeams: jest.fn(),
          setCurrentTeam: jest.fn(),
        },
      },
    });
    const { user } = render(
      <QueryClientProvider client={queryClient}>
        <LoginPage
          router={router}
          location={createMockLocation({ pathname: "/login" })}
        />
      </QueryClientProvider>
    );

    await user.type(await screen.findByLabelText("Email"), "next@example.com");
    await user.type(screen.getByLabelText("Password"), "password");
    await user.click(screen.getByRole("button", { name: "Log in" }));

    await waitFor(() => {
      expect(router.push).toHaveBeenCalledWith(paths.DASHBOARD);
    });
    expect(queryClient.getQueryData([1, 7])).toBeUndefined();
  });
});

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
