export type E2EState = {
  sidUnit?: string;
  sidInteg?: string;
  sidLint?: string;
  sidLs?: string;
  sidLsAll?: string;
  sidLsDirs?: string;
  tUnit?: string;
  tList?: string;
  bb1?: string;
  bb2?: string;
  st1?: string;
  st2?: string;
  st3?: string;
  convID?: string;
  expID?: string;
};

export type E2EContext = {
  TEST_ROLE_USER: string;
  TEST_ROLE_QA: string;
  SKIP_RESET: boolean;
  SKIP_SNAPSHOT: boolean;
  total: number;
  step: number;
  state: E2EState;
  nextStep: (msg: string) => void;
};
