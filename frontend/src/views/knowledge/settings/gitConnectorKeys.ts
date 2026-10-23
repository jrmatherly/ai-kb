// The git connectors share one repository-row form; each names its settings keys and label group.
export type GitConnectorType = 'gitlab' | 'github' | 'bitbucket'

export const GIT_CONNECTOR_KEYS: Record<GitConnectorType, { settings: string; item: string; labels: string }> = {
  gitlab: { settings: 'projects', item: 'project_id', labels: 'datasource.gitlab' },
  github: { settings: 'repositories', item: 'repository', labels: 'datasource.github' },
  bitbucket: { settings: 'repositories', item: 'repository', labels: 'datasource.bitbucket' },
}

export const isGitConnector = (type: string): type is GitConnectorType => Object.hasOwn(GIT_CONNECTOR_KEYS, type)
