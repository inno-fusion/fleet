import { screen } from "@testing-library/react";
import React from "react";

import { createMockConfig, createMockMdmConfig } from "__mocks__/configMock";
import createMockLicense from "__mocks__/licenseMock";
import { createCustomRenderer } from "test/test-utils";

import MainContent from "./MainContent";

const GRAPH_BANNER_TEXT =
  "Your Microsoft Graph client secret is expired, deleted, or has missing " +
  "permissions. Windows Autopilot devices won't sync to Fleet as pending " +
  "hosts. Users with the admin role in Fleet can update the credential.";

const renderMainContent = ({
  credentialInvalid = false,
  isVppExpired = false,
  tier = "premium",
  expiration = "2050-01-01T00:00:00Z",
} = {}) => {
  const render = createCustomRenderer({
    context: {
      // Every banner but the APNs one sits inside MainContent's premium gate; these all assume premium.
      app: {
        isPremiumTier: true,
        isVppExpired,
        config: createMockConfig({
          license: createMockLicense({ tier, expiration }),
          mdm: createMockMdmConfig({
            microsoft_graph_credential_invalid: credentialInvalid,
          }),
        }),
      },
    },
  });

  return render(
    <MainContent>
      <div>content</div>
    </MainContent>
  );
};

describe("MainContent app-wide banners", () => {
  it("does not treat local research's absent expiry as an expired subscription", () => {
    renderMainContent({ tier: "research", expiration: "0001-01-01T00:00:00Z" });
    expect(
      screen.queryByText(/Premium license is about to expire/i)
    ).not.toBeInTheDocument();
  });

  it("still warns when a signed Premium license has expired", () => {
    renderMainContent({ expiration: "2020-01-01T00:00:00Z" });
    expect(
      screen.getByText(/Premium license is about to expire/i)
    ).toBeInTheDocument();
  });

  it("still warns about invalid integration credentials in research mode", () => {
    renderMainContent({ tier: "research", credentialInvalid: true });
    expect(screen.getByText(GRAPH_BANNER_TEXT)).toBeInTheDocument();
  });

  it("shows the Microsoft Graph banner when the credential is invalid", () => {
    renderMainContent({ credentialInvalid: true });
    expect(screen.getByText(GRAPH_BANNER_TEXT)).toBeInTheDocument();
  });

  it("does not show the banner when the credential is healthy", () => {
    renderMainContent({ credentialInvalid: false });
    expect(screen.queryByText(GRAPH_BANNER_TEXT)).not.toBeInTheDocument();
  });

  it("yields to the VPP banner, which sits above it in the priority order", () => {
    // Only one banner renders at a time.
    renderMainContent({ credentialInvalid: true, isVppExpired: true });
    expect(screen.queryByText(GRAPH_BANNER_TEXT)).not.toBeInTheDocument();
    expect(
      screen.getByText(/Volume Purchasing Program \(VPP\) content token/i)
    ).toBeInTheDocument();
  });
});
