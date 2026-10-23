import assert from 'node:assert/strict'
import test from 'node:test'

import { GIT_CONNECTOR_KEYS, isGitConnector } from './gitConnectorKeys'

test('the git connector table covers every git type', () => {
  assert.deepEqual(Object.keys(GIT_CONNECTOR_KEYS).sort(), ['bitbucket', 'github', 'gitlab'])
  assert.equal(isGitConnector('bitbucket'), true)
  assert.equal(isGitConnector('seafile'), false)
  assert.deepEqual(GIT_CONNECTOR_KEYS.bitbucket, { settings: 'repositories', item: 'repository', labels: 'datasource.bitbucket' })
})
