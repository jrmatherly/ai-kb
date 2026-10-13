-- Migration script: move from PostgreSQL to ParadeDB.
-- Back up the data before running this script.

-- 1. Export the data (run on PostgreSQL)
-- pg_dump -U postgres -h localhost -p 5432 -d your_database > backup.sql

-- 2. Import the data (run on ParadeDB)
-- psql -U postgres -h localhost -p 5432 -d your_database < backup.sql

-- 3. Verify the data


-- Insert some sample data
-- INSERT INTO tenants (id, name, description, status, api_key)
-- VALUES 
--     (1, 'Demo Tenant', 'This is a demo tenant for testing', 'active', 'sk-00000001abcdefg123456')
-- ON CONFLICT DO NOTHING;

-- SELECT setval('tenants_id_seq', (SELECT MAX(id) FROM tenants));


-- -- Create knowledge base
-- INSERT INTO knowledge_bases (id, name, description, tenant_id, chunking_config, image_processing_config, embedding_model_id)
-- VALUES 
--     ('kb-00000001', 'Default Knowledge Base', 'Default knowledge base for testing', 1, '{"chunk_size": 512, "chunk_overlap": 50, "separators": ["\n\n", "\n", "。"], "keep_separator": true}', '{"enable_multimodal": false, "model_id": ""}', 'model-embedding-00000001'),
--     ('kb-00000002', 'Test Knowledge Base', 'Test knowledge base for development', 1, '{"chunk_size": 512, "chunk_overlap": 50, "separators": ["\n\n", "\n", "。"], "keep_separator": true}', '{"enable_multimodal": false, "model_id": ""}', 'model-embedding-00000001'),
--     ('kb-00000003', 'Test Knowledge Base 2', 'Test knowledge base for development 2', 1, '{"chunk_size": 512, "chunk_overlap": 50, "separators": ["\n\n", "\n", "。"], "keep_separator": true}', '{"enable_multimodal": false, "model_id": ""}', 'model-embedding-00000001')
-- ON CONFLICT DO NOTHING;


SELECT COUNT(*) FROM tenants;
SELECT COUNT(*) FROM models;
SELECT COUNT(*) FROM knowledge_bases;
SELECT COUNT(*) FROM knowledges;


-- Test Chinese full-text search

-- Create the documents table
CREATE TABLE chinese_documents (
    id SERIAL PRIMARY KEY,
    title TEXT,
    content TEXT,
    published_date DATE
);

-- BM25 index with a Chinese tokenizer (chinese_lindera)
CREATE INDEX idx_documents_bm25 ON chinese_documents
USING bm25 (id, content)
WITH (
    key_field = 'id',
    text_fields = '{
        "content": {
          "tokenizer": {"type": "chinese_lindera"}
        }
    }'
);

-- Chinese sample rows for the tokenizer, written as U& escapes (needs standard_conforming_strings on)
INSERT INTO chinese_documents (title, content, published_date)
VALUES 
(U&'\4EBA\5DE5\667A\80FD\7684\53D1\5C55', U&'\4EBA\5DE5\667A\80FD\6280\672F\6B63\5728\5FEB\901F\53D1\5C55\FF0C\5F71\54CD\4E86\6211\4EEC\751F\6D3B\7684\65B9\65B9\9762\9762\3002\5927\8BED\8A00\6A21\578B\662F\6700\8FD1\7684\4E00\4E2A\91CD\8981\7A81\7834\3002', '2023-01-15'),
(U&'\673A\5668\5B66\4E60\57FA\7840', U&'\673A\5668\5B66\4E60\662F\4EBA\5DE5\667A\80FD\7684\4E00\4E2A\91CD\8981\5206\652F\FF0C\5305\62EC\76D1\7763\5B66\4E60\3001\65E0\76D1\7763\5B66\4E60\548C\5F3A\5316\5B66\4E60\7B49\65B9\6CD5\3002', '2023-02-20'),
(U&'\6DF1\5EA6\5B66\4E60\5E94\7528', U&'\6DF1\5EA6\5B66\4E60\5728\56FE\50CF\8BC6\522B\3001\81EA\7136\8BED\8A00\5904\7406\548C\8BED\97F3\8BC6\522B\7B49\9886\57DF\6709\5E7F\6CDB\5E94\7528\3002', '2023-03-10'),
(U&'\81EA\7136\8BED\8A00\5904\7406\6280\672F', U&'\81EA\7136\8BED\8A00\5904\7406\5141\8BB8\8BA1\7B97\673A\7406\89E3\3001\89E3\91CA\548C\751F\6210\4EBA\7C7B\8BED\8A00\FF0C\662F\4EBA\5DE5\667A\80FD\7684\6838\5FC3\6280\672F\4E4B\4E00\3002', '2023-04-05'),
(U&'\8BA1\7B97\673A\89C6\89C9\5165\95E8', U&'\8BA1\7B97\673A\89C6\89C9\8BA9\673A\5668\80FD\591F"\770B\5230"\5E76\7406\89E3\89C6\89C9\4E16\754C\FF0C\5E7F\6CDB\5E94\7528\4E8E\5B89\9632\3001\533B\7597\7B49\9886\57DF\3002', '2023-05-12');

INSERT INTO chinese_documents (title, content, published_date)
VALUES 
('hello world', 'hello world', '2023-05-12');
