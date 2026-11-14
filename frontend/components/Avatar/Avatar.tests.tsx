import { render, screen } from "@testing-library/react";
import React from "react";

import Avatar from "./Avatar";

describe("Avatar - component", () => {
  it("renders the user gravatar if provided", () => {
    render(
      <Avatar user={{ gravatar_url: "https://example.com/avatar.png" }} />
    );

    const avatar = screen.getByAltText("User avatar");
    expect(avatar).toBeVisible();
    expect(avatar).toHaveAttribute("src", "https://example.com/avatar.png");
  });

  it.each([
    "https://www.gravatar.com/avatar/email-hash",
    "//secure.gravatar.com/avatar/email-hash",
    "https://fleetdm.com/images/permanent/icon-avatar-default-gray-transparent-64x64@2x.png",
  ])("renders a local default instead of a legacy vendor avatar: %s", (url) => {
    render(<Avatar user={{ gravatar_url: url }} />);
    expect(screen.getByTestId("default-avatar")).toBeVisible();
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
  });

  it("renders a local default when no avatar is supplied", () => {
    render(<Avatar user={{}} />);
    expect(screen.getByTestId("default-avatar")).toBeVisible();
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
  });

  it("renders an API-only avatar without requesting a supplied image", () => {
    render(
      <Avatar
        useApiOnlyAvatar
        user={{ gravatar_url: "https://images.example.com/avatar.png" }}
      />
    );
    expect(screen.getByTestId("apionly-avatar")).toBeVisible();
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
  });

  it("renders the fleet avatar if useFleetAvatar is `true`", () => {
    render(<Avatar useFleetAvatar user={{}} />);
    expect(screen.getByTestId("fleet-avatar")).toBeVisible();
  });
});
