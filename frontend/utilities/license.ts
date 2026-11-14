import { ILicense } from "interfaces/config";

type LicenseTier = Pick<ILicense, "tier"> | null | undefined;

// Match LicenseInfo.IsPremium on the server. This controls local feature
// availability, not authorization or access to Fleet-hosted services.
export const hasPremiumFeatures = (license: LicenseTier): boolean => {
  return ["premium", "basic", "trial", "research"].includes(
    license?.tier || ""
  );
};

export const isResearchLicense = (license: LicenseTier): boolean => {
  return license?.tier === "research";
};
