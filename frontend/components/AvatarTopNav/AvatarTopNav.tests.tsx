import { render, screen } from "@testing-library/react";
import React from "react";

import {
  DEFAULT_GRAVATAR_LINK,
  DEFAULT_GRAVATAR_LINK_DARK,
  DEFAULT_GRAVATAR_LINK_FALLBACK,
  DEFAULT_GRAVATAR_LINK_DARK_FALLBACK,
} from "utilities/constants";

import Avatar from "./AvatarTopNav";

describe("AvatarTopNav", () => {
  it("uses bundled defaults, including compatibility aliases", () => {
    [
      DEFAULT_GRAVATAR_LINK,
      DEFAULT_GRAVATAR_LINK_DARK,
      DEFAULT_GRAVATAR_LINK_FALLBACK,
      DEFAULT_GRAVATAR_LINK_DARK_FALLBACK,
    ].forEach((url) => {
      expect(url).not.toMatch(/^(https?:)?\/\//);
      expect(url).toBeTruthy();
    });
  });

  it.each([
    undefined,
    "https://www.gravatar.com/avatar/email-hash",
    "//secure.gravatar.com/avatar/email-hash",
    "https://fleetdm.com/images/permanent/icon-avatar-default-dark-24x24%402x.png",
  ])("uses a local default for absent or legacy vendor avatars: %s", (url) => {
    render(<Avatar user={{ gravatar_url: url }} size="small" />);
    expect(screen.getByAltText("User avatar")).toHaveAttribute(
      "src",
      DEFAULT_GRAVATAR_LINK
    );
  });

  it("preserves explicitly configured custom avatars", () => {
    render(
      <Avatar
        user={{ gravatar_url: "https://images.example.com/avatar.png" }}
      />
    );
    expect(screen.getByAltText("User avatar")).toHaveAttribute(
      "src",
      "https://images.example.com/avatar.png"
    );
  });
});
