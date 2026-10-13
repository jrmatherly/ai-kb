import assert from 'node:assert/strict'
import test from 'node:test'

import {
  DEFAULT_RESOURCE_SORT,
  RESOURCE_SORT_OPTIONS,
  getResourceSortOption,
  sortResourceItems,
  sortResourcesWithinGroups,
  type ResourceSortAccessors,
} from './resourceSorting'

interface Item {
  id: string
  group: 'mine' | 'shared'
  name?: string
  created_at?: string
  updated_at?: string
}

const accessors: ResourceSortAccessors<Item> = {
  getName: item => item.name,
  getCreatedAt: item => item.created_at,
  getUpdatedAt: item => item.updated_at,
}

const items: Item[] = [
  { id: 'old-z', group: 'mine', name: 'Zulu 10', created_at: '2024-01-01', updated_at: '2024-03-01' },
  { id: 'new-a', group: 'mine', name: 'Alpha 2', created_at: '2024-02-01', updated_at: '2024-04-01' },
  { id: 'middle-a', group: 'mine', name: 'Alpha 11', created_at: '2024-01-15', updated_at: '2024-03-15' },
]

test('resource sorting defaults to updated time descending and offers three groups of six options', () => {
  assert.equal(DEFAULT_RESOURCE_SORT, 'updated_desc')
  assert.deepEqual(
    RESOURCE_SORT_OPTIONS.map(({ value, sortBy, sortOrder }) => [value, sortBy, sortOrder]),
    [
      ['updated_desc', 'updated_at', 'desc'],
      ['updated_asc', 'updated_at', 'asc'],
      ['created_desc', 'created_at', 'desc'],
      ['created_asc', 'created_at', 'asc'],
      ['name_asc', 'name', 'asc'],
      ['name_desc', 'name', 'desc'],
    ],
  )
  assert.deepEqual(
    sortResourceItems(items, DEFAULT_RESOURCE_SORT, accessors).map(item => item.id),
    ['new-a', 'middle-a', 'old-z'],
  )
})

test('an unknown sort value falls back to the default option safely', () => {
  assert.equal(getResourceSortOption('invalid' as never).value, DEFAULT_RESOURCE_SORT)
})

test('resources sort by created time and by name in both directions', () => {
  assert.deepEqual(
    sortResourceItems(items, 'created_asc', accessors).map(item => item.id),
    ['old-z', 'middle-a', 'new-a'],
  )
  assert.deepEqual(
    sortResourceItems(items, 'name_asc', accessors).map(item => item.id),
    ['new-a', 'middle-a', 'old-z'],
  )
  assert.deepEqual(
    sortResourceItems(items, 'name_desc', accessors).map(item => item.id),
    ['old-z', 'middle-a', 'new-a'],
  )
})

test('sorting keeps the fixed group order and reorders only within each group', () => {
  const mixed: Item[] = [
    { ...items[0], group: 'shared' },
    { ...items[1], group: 'mine' },
    { ...items[2], group: 'shared' },
  ]
  assert.deepEqual(
    sortResourcesWithinGroups(mixed, 'updated_desc', item => item.group, ['mine', 'shared'], accessors)
      .map(item => `${item.group}:${item.id}`),
    ['mine:new-a', 'shared:middle-a', 'shared:old-z'],
  )
})

test('legacy rows missing the sort field always sort last, and equal values keep their original order', () => {
  const legacy: Item[] = [
    { id: 'first', group: 'mine', name: 'Same', updated_at: '2024-01-01' },
    { id: 'missing', group: 'mine' },
    { id: 'second', group: 'mine', name: 'same', updated_at: '2024-01-01' },
  ]
  assert.deepEqual(
    sortResourceItems(legacy, 'updated_asc', accessors).map(item => item.id),
    ['first', 'second', 'missing'],
  )
  assert.deepEqual(
    sortResourceItems(legacy, 'name_desc', accessors).map(item => item.id),
    ['first', 'second', 'missing'],
  )
})
