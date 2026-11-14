import { screen, waitFor } from "@testing-library/react";
import React from "react";

import createMockConfig from "__mocks__/configMock";
import createMockLicense from "__mocks__/licenseMock";
import createMockUser from "__mocks__/userMock";
import versionAPI from "services/entities/version";
import { createCustomRenderer } from "test/test-utils";

import AccountSidePanel from "./AccountSidePanel";

jest.mock("services/entities/version", () => ({
  __esModule: true,
  default: { load: jest.fn() },
}));

const renderAccount = (tier: string) => {
  jest.mocked(versionAPI.load).mockResolvedValue({
    version: "test",
    branch: "test",
    revision: "test",
    go_version: "test",
    build_date: "",
    build_user: "",
  });
  const render = createCustomRenderer({
    context: {
      app: {
        isPremiumTier: true,
        config: createMockConfig({
          license: createMockLicense({
            tier,
            expiration:
              tier === "research"
                ? "0001-01-01T00:00:00Z"
                : "2050-01-01T00:00:00Z",
          }),
        }),
      },
    },
  });
  return render(
    <AccountSidePanel
      currentUser={createMockUser()}
      onChangePassword={jest.fn()}
      onGetApiToken={jest.fn()}
    />
  );
};

describe("AccountSidePanel license metadata", () => {
  it("labels local research honestly instead of showing a year-one expiration date", async () => {
    renderAccount("research");
    expect(
      screen.queryByText("Change photo at Gravatar")
    ).not.toBeInTheDocument();
    expect(screen.getByText("Feature entitlement")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Local research (development/testing only, no expiration)"
      )
    ).toBeInTheDocument();
    expect(
      screen.queryByText("License expiration date")
    ).not.toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByText(/Fleet test/)).toBeInTheDocument()
    );
  });

  it("preserves the signed license expiration display", async () => {
    renderAccount("premium");
    expect(screen.getByText("License expiration date")).toBeInTheDocument();
    expect(screen.queryByText("Feature entitlement")).not.toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByText(/Fleet test/)).toBeInTheDocument()
    );
  });
});
