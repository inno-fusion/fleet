import { screen } from "@testing-library/react";
import React from "react";
import { ErrorBoundary } from "react-error-boundary";

import createMockConfig from "__mocks__/configMock";
import createMockUser from "__mocks__/userMock";
import paths from "router/paths";
import { createMockRouter, renderWithAppContext } from "test/test-utils";
import authToken from "utilities/auth_token";

import AuthenticatedRoutes from "./AuthenticatedRoutes";

describe("AuthenticatedRoutes privacy", () => {
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
  });

  it.each([false, true])(
    "does not initialize or track users even when an SDK is present (sandbox: %s)",
    (isSandboxMode) => {
      const router = createMockRouter();
      renderWithAppContext(
        <ErrorBoundary fallback={<div>Error</div>}>
          <AuthenticatedRoutes router={router} location={{ hash: "" }}>
            <div>Authenticated page</div>
          </AuthenticatedRoutes>
        </ErrorBoundary>,
        {
          contextValue: {
            currentUser: createMockUser(),
            config: createMockConfig({ sandbox_enabled: isSandboxMode }),
            isSandboxMode,
          },
        }
      );

      expect(screen.getByText("Authenticated page")).toBeVisible();
      expect(initialize).not.toHaveBeenCalled();
      expect(track).not.toHaveBeenCalled();
      expect(router.push).not.toHaveBeenCalled();
    }
  );

  it("still redirects a missing session to the local login page", () => {
    jest.mocked(authToken.get).mockReturnValue(null);
    const router = createMockRouter();
    renderWithAppContext(
      <ErrorBoundary fallback={<div>Error</div>}>
        <AuthenticatedRoutes router={router} location={{ hash: "" }}>
          <div>Authenticated page</div>
        </AuthenticatedRoutes>
      </ErrorBoundary>,
      { contextValue: { currentUser: null } }
    );
    expect(router.push).toHaveBeenCalledWith(paths.LOGIN);
    expect(initialize).not.toHaveBeenCalled();
  });
});
