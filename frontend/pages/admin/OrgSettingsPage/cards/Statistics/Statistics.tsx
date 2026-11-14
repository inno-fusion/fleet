import React from "react";

import Checkbox from "components/forms/fields/Checkbox";
import PageDescription from "components/PageDescription";
import SettingsSection from "pages/admin/components/SettingsSection";

import { IAppConfigFormProps } from "../constants";

const baseClass = "app-config-form";

const Statistics: React.FC<IAppConfigFormProps> = () => (
  <SettingsSection title="Usage statistics">
    <PageDescription
      variant="right-panel"
      content={
        <span className={`${baseClass}__section-description`}>
          Fleet product analytics are permanently disabled in this build. No
          usage statistics are collected or sent to Fleet, including on Premium.
          <br />
          <br />
          Device inventory, MDM, audit logs, historical reporting, and
          operator-configured metrics and error reporting are unaffected.
        </span>
      }
    />
    <Checkbox name="enableUsageStatistics" value={false} disabled>
      Enable usage statistics
    </Checkbox>
  </SettingsSection>
);

export default Statistics;
