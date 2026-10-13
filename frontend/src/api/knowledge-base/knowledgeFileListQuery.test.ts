import assert from 'node:assert/strict';
import test from 'node:test';

import { buildListKnowledgeFilesQuery } from './knowledgeFileListQuery';

test('the knowledge file list request passes the sort field and direction', () => {
  const query = new URLSearchParams(buildListKnowledgeFilesQuery({
    page: 2,
    page_size: 35,
    sort_by: 'file_name',
    sort_order: 'asc',
    folder_path: '',
  }));

  assert.equal(query.get('page'), '2');
  assert.equal(query.get('page_size'), '35');
  assert.equal(query.get('sort_by'), 'file_name');
  assert.equal(query.get('sort_order'), 'asc');
  assert.equal(query.get('folder_path'), '');
});
