import type { IRegistrationFormData } from "interfaces/registration_form_data";
import { getPastDate, getFutureDate } from "test/test-utils";

import helpers, {
  removeOSPrefix,
  compareVersions,
  willExpireWithinXDays,
  humanLastSeen,
  internationalTimeOnlyFormat,
} from "./helpers";

describe("helpers utilities", () => {
  describe("addGravatarUrlToResource function", () => {
    it.each([undefined, "true", "false"])(
      "does not derive an external avatar from an email (availability: %s)",
      (availability) => {
        if (availability === undefined) {
          localStorage.removeItem("gravatar_available");
        } else {
          localStorage.setItem("gravatar_available", availability);
        }
        const resource = { email: "Admin@Example.com", name: "Admin" };
        const result = helpers.addGravatarUrlToResource(resource);

        expect(result).toEqual({
          ...resource,
          gravatar_url: undefined,
          gravatar_url_dark: undefined,
        });
        expect(resource).not.toHaveProperty("gravatar_url");
        localStorage.removeItem("gravatar_available");
      }
    );

    it.each([
      "https://www.gravatar.com/avatar/email-hash",
      "https://secure.gravatar.com/avatar/email-hash",
      "//GRAVATAR.com/avatar/email-hash",
      "https://fleetdm.com/images/permanent/icon-avatar-default-gray-transparent-64x64@2x.png",
      "https://www.fleetdm.com/images/permanent/icon-avatar-default-dark-24x24%402x.png",
    ])("discards a legacy vendor avatar: %s", (url) => {
      expect(
        helpers.addGravatarUrlToResource({
          gravatar_url: url,
          gravatar_url_dark: url,
        })
      ).toEqual({ gravatar_url: undefined, gravatar_url_dark: undefined });
    });

    it.each([
      "https://images.example.com/avatar.png",
      "https://gravatar.com.example.com/avatar.png",
      "https://fleetdm.com/custom-avatar.png",
      "/assets/custom-avatar.png",
      "data:image/png;base64,aGVsbG8=",
    ])("preserves an explicitly configured custom avatar: %s", (url) => {
      const resource = { gravatar_url: url, gravatar_url_dark: url };
      expect(helpers.addGravatarUrlToResource(resource)).toEqual(resource);
    });

    it("discards malformed avatar URLs without affecting the user resource", () => {
      expect(
        helpers.addGravatarUrlToResource({
          name: "Admin",
          gravatar_url: "https://[invalid",
        })
      ).toEqual({
        name: "Admin",
        gravatar_url: undefined,
        gravatar_url_dark: undefined,
      });
    });
  });

  describe("removeOSPrefix function", () => {
    it("properly removes Apple prefix from a host's operating system version", () => {
      expect(removeOSPrefix("macOS 14.1.2")).toEqual("14.1.2");
      expect(removeOSPrefix("iOS 18.0")).toEqual("18.0");
      expect(removeOSPrefix("iPadOS 17.5.1")).toEqual("17.5.1");
    });
  });

  describe("compareVersions function", () => {
    it("properly checks if a version is older than another", () => {
      expect(compareVersions("14.4.1", "14.4.2")).toEqual(-1);
      expect(compareVersions("14.4.1", "14.5")).toEqual(-1);
      expect(compareVersions("14.4.1", "15")).toEqual(-1);

      expect(compareVersions("14.4", "14.4.2")).toEqual(-1);
      expect(compareVersions("14.4", "14.5")).toEqual(-1);
      expect(compareVersions("14.4", "15")).toEqual(-1);

      expect(compareVersions("14", "14.4.2")).toEqual(-1);
      expect(compareVersions("14", "14.0.5")).toEqual(-1);
      expect(compareVersions("14", "15")).toEqual(-1);
    });

    it("properly checks if a version is newer than another", () => {
      expect(compareVersions("14.4.4", "14.4.3")).toEqual(1);
      expect(compareVersions("14.3.4", "14.3")).toEqual(1);
      expect(compareVersions("14.0.4", "14")).toEqual(1);

      expect(compareVersions("14.5", "14.4.3")).toEqual(1);
      expect(compareVersions("14.5", "14.3")).toEqual(1);
      expect(compareVersions("14.5", "14")).toEqual(1);

      expect(compareVersions("14", "13.9.21")).toEqual(1);
      expect(compareVersions("14", "13.9")).toEqual(1);
      expect(compareVersions("14", "13")).toEqual(1);
    });

    it("properly checks if a version is equal to another", () => {
      expect(compareVersions("14.0.4", "14.0.4")).toEqual(0);
      expect(compareVersions("14.3", "14.3")).toEqual(0);
      expect(compareVersions("14", "14")).toEqual(0);
      expect(compareVersions("14.3", "14.3.0")).toEqual(0);
      expect(compareVersions("14", "14.0.0")).toEqual(0);
    });
  });

  describe("willExpireWithinXDays function", () => {
    it("will return true if the date is within x number of days", () => {
      const fiveDaysFromNow = getFutureDate(5);
      expect(willExpireWithinXDays(fiveDaysFromNow, 10)).toEqual(true);

      const tenDaysFromNow = getFutureDate(10);
      expect(willExpireWithinXDays(tenDaysFromNow, 30)).toEqual(true);
    });

    it("will return false if the date is not within x number of days", () => {
      const thirtyDaysFromNow = getFutureDate(30);
      expect(willExpireWithinXDays(thirtyDaysFromNow, 10)).toEqual(false);

      const fiftyDaysFromNow = getFutureDate(50);
      expect(willExpireWithinXDays(fiftyDaysFromNow, 30)).toEqual(false);
    });

    it("will return false if the date has already expired", () => {
      const fiveDaysAgo = getPastDate(5);
      expect(willExpireWithinXDays(fiveDaysAgo, 10)).toEqual(false);

      const fiftyDaysAgo = getPastDate(50);
      expect(willExpireWithinXDays(fiftyDaysAgo, 30)).toEqual(false);
    });
  });

  describe("humanLastSeen function", () => {
    beforeEach(() => {
      jest.useFakeTimers().setSystemTime(new Date("2026-06-15T12:00:00Z"));
    });

    afterEach(() => {
      jest.useRealTimers();
    });

    it("uses days below the month threshold", () => {
      expect(humanLastSeen(getPastDate(5))).toEqual("5 days ago");
      expect(humanLastSeen(getPastDate(89))).toEqual("89 days ago");
    });

    it("uses months at or beyond 90 days", () => {
      expect(humanLastSeen(getPastDate(90))).toEqual("3 months ago");
      expect(humanLastSeen(getPastDate(100))).toEqual("3 months ago");
    });
  });

  describe("setupData function", () => {
    it("excludes the org logo file from the JSON setup payload", () => {
      const formData: IRegistrationFormData = {
        email: "admin@example.com",
        name: "Admin",
        password: "password123",
        password_confirmation: "password123",
        org_name: "Fleet",
        org_web_url: "",
        org_logo_file: new File(["x"], "logo.png", { type: "image/png" }),
        fleet_web_address: "",
        server_url: "https://fleet.example.com",
      };

      const result = helpers.setupData(formData);

      expect(result.org_info).toEqual({ org_name: "Fleet" });
    });
  });

  describe("internationalTimeOnlyFormat function", () => {
    const setLanguage = (lang: string) => {
      Object.defineProperty(window.navigator, "languages", {
        value: [lang],
        configurable: true,
      });
    };

    let originalLanguages: readonly string[];
    beforeAll(() => {
      originalLanguages = window.navigator.languages;
    });
    afterAll(() => {
      Object.defineProperty(window.navigator, "languages", {
        value: originalLanguages,
        configurable: true,
      });
    });

    it("renders 24-hour source times in 12-hour form for US locale", () => {
      setLanguage("en-US");
      // A narrow no-break space (U+202F) or NBSP (U+00A0) may appear
      // between the number and AM/PM in some ICU versions; match any
      // whitespace via \\s? to stay portable across Node/ICU versions.
      expect(internationalTimeOnlyFormat("02:00")).toMatch(/2:00\s?AM/i);
      expect(internationalTimeOnlyFormat("14:30")).toMatch(/2:30\s?PM/i);
    });

    it("keeps 24-hour form for a 24-hour locale (de-DE)", () => {
      setLanguage("de-DE");
      // German locale uses 24-hour; either 02:00 or 2:00 is acceptable
      // depending on ICU version, but always without AM/PM.
      expect(internationalTimeOnlyFormat("02:00")).toMatch(/^0?2:00$/);
      expect(internationalTimeOnlyFormat("14:30")).toMatch(/^14:30$/);
    });

    it("passes through an unparseable string unchanged", () => {
      setLanguage("en-US");
      expect(internationalTimeOnlyFormat("nope")).toBe("nope");
      expect(internationalTimeOnlyFormat("")).toBe("");
      expect(internationalTimeOnlyFormat("2:")).toBe("2:");
    });

    it("passes through out-of-range times unchanged", () => {
      setLanguage("en-US");
      // These would otherwise silently wrap via Date.setHours (24:00 -> next day).
      // Passthrough keeps the tooltip honest about bad data instead of hiding it.
      expect(internationalTimeOnlyFormat("24:00")).toBe("24:00");
      expect(internationalTimeOnlyFormat("10:60")).toBe("10:60");
    });

    it("handles edge times 00:00 and 23:59", () => {
      setLanguage("en-US");
      expect(internationalTimeOnlyFormat("00:00")).toMatch(/12:00\s?AM/i);
      expect(internationalTimeOnlyFormat("23:59")).toMatch(/11:59\s?PM/i);
    });
  });
});
