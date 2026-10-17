// Copied from frontend/src/utils/tool-icons.ts (toolDisplayNameKeys) with the en-US strings (tools.*).
const LABELS: Record<string, string> = {
  multi_kb_search: 'Cross-KB Search',
  search_knowledge: 'Search knowledge',
  read_document: 'Read document',
  list_documents: 'List documents',
  get_chunk_detail: 'Get Chunk Detail',
  list_knowledge_bases: 'List Knowledge Bases',
  query_knowledge_graph: 'Query Knowledge Graph',
  think: 'Deep Thinking',
  todo_write: 'Make Plan',
  knowledge_search: 'Knowledge Search',
  grep_chunks: 'Text Pattern Search',
  list_knowledge_chunks: 'List Knowledge Chunks',
  get_document_info: 'Get Document Info',
};

export const toolLabel = (toolName: string): string => LABELS[toolName] ?? toolName.replace(/_/g, ' ');
