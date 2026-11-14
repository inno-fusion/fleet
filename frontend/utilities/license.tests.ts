import { ILicense } from "interfaces/config";

import { hasPremiumFeatures, isResearchLicense } from "./license";

const license = (tier: string): Pick<ILicense, "tier"> => ({ tier });

describe("local feature entitlements", () => {
  it.each(["premium", "basic", "trial", "research"])(
    "enables implemented Premium features for %s",
    (tier) => {
      expect(hasPremiumFeatures(license(tier))).toBe(true);
    }
  );

  it.each(["free", "", "unrecognized", "RESEARCH"])(
    "does not infer feature availability for %s",
    (tier) => {
      expect(hasPremiumFeatures(license(tier))).toBe(false);
    }
  );

  it("does not enable features before a license is loaded", () => {
    expect(hasPremiumFeatures(undefined)).toBe(false);
    expect(hasPremiumFeatures(null)).toBe(false);
  });

  it("distinguishes local research from a signed subscription or trial", () => {
    expect(isResearchLicense(license("research"))).toBe(true);
    ["premium", "basic", "trial", "free", ""].forEach((tier) => {
      expect(isResearchLicense(license(tier))).toBe(false);
    });
    expect(isResearchLicense(undefined)).toBe(false);
    expect(isResearchLicense(null)).toBe(false);
  });
});
