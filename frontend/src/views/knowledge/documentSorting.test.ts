import assert from 'node:assert/strict';
import test from 'node:test';

import {
  DEFAULT_DOCUMENT_SORT,
  DOCUMENT_SORT_OPTIONS,
  getDocumentSortOption,
  getDocumentSortParams,
} from './documentSorting';

test('document sorting defaults to creation time, newest first', () => {
  assert.equal(DEFAULT_DOCUMENT_SORT, 'created_desc');
  assert.deepEqual(getDocumentSortParams(DEFAULT_DOCUMENT_SORT), {
    sort_by: 'created_at',
    sort_order: 'desc',
  });
});

test('document sorting offers three groups of six options mapped to the server allowlist parameters', () => {
  assert.equal(DOCUMENT_SORT_OPTIONS.length, 6);
  assert.deepEqual(
    DOCUMENT_SORT_OPTIONS.map(({ value, sortBy, sortOrder }) => [value, sortBy, sortOrder]),
    [
      ['updated_desc', 'updated_at', 'desc'],
      ['updated_asc', 'updated_at', 'asc'],
      ['created_desc', 'created_at', 'desc'],
      ['created_asc', 'created_at', 'asc'],
      ['name_asc', 'file_name', 'asc'],
      ['name_desc', 'file_name', 'desc'],
    ],
  );
});

test('an unknown sort value falls back safely to the default option', () => {
  assert.equal(getDocumentSortOption('invalid' as never).value, DEFAULT_DOCUMENT_SORT);
});
