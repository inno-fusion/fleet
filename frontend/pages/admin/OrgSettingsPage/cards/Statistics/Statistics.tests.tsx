import { screen } from "@testing-library/react";
import React from "react";

import createMockConfig from "__mocks__/configMock";
import { createCustomRenderer, createMockRouter } from "test/test-utils";

import Statistics from "./Statistics";

it.each([false, true])(
  "never offers analytics opt-in, even with stale enabled config (Premium: %s)",
  async (isPremiumTier) => {
    const config = createMockConfig();
    config.server_settings.enable_analytics = true;
    config.license.allow_disable_telemetry = false;
    const handleSubmit = jest.fn();
    const render = createCustomRenderer({
      context: { app: { isPremiumTier } },
    });
    const { user } = render(
      <Statistics
        appConfig={config}
        handleSubmit={handleSubmit}
        isPremiumTier={isPremiumTier}
        isUpdatingSettings={false}
        router={createMockRouter()}
      />
    );

    const checkbox = screen.getByRole("checkbox");
    expect(checkbox).not.toBeChecked();
    expect(checkbox).toHaveAttribute("aria-disabled", "true");
    expect(screen.getByText(/permanently disabled/i)).toBeInTheDocument();
    expect(
      screen.getByText(/Device inventory, MDM, audit logs/i)
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /save/i })
    ).not.toBeInTheDocument();
    await user.click(checkbox);
    expect(checkbox).not.toBeChecked();
    expect(handleSubmit).not.toHaveBeenCalled();
  }
);
