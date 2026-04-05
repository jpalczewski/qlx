import { test } from '../fixtures/app';

test.describe('Quick-entry description', () => {
  test.skip('removed - tokenized quick entry does not have inline description', () => {
    // The old quick entry had a collapsible description field.
    // The new tokenized input does not have this feature.
  });
});
