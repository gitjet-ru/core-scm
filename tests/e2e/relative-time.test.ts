import {test, expect} from '@playwright/test';
import {assertNoJsError} from './utils.ts';

test('relative-time renders without errors', async ({page}) => {
  const response = await page.goto('/devtest/relative-time');
  test.skip(!response || response.status() === 404, 'devtest routes are disabled in this deployment profile');
  const relativeTime = page.getByTestId('relative-time-now');
  await expect(relativeTime).toHaveAttribute('data-tooltip-content', /.+/);
  await expect(relativeTime).toHaveText('now');
  await assertNoJsError(page);
});
