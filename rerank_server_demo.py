import gc
import torch
import uvicorn
from fastapi import FastAPI
from pydantic import BaseModel, Field
from transformers import AutoModelForSequenceClassification, AutoTokenizer
from typing import List

# Enable CUDA debugging
# import os
# os.environ['CUDA_LAUNCH_BLOCKING']='1'

# --- 1. Define the API request and response data structures ---

# The request body structure is unchanged
class RerankRequest(BaseModel):
    query: str
    documents: List[str]

# --- Change start: test response structure with the field name "score" ---

# The DocumentInfo structure is unchanged
class DocumentInfo(BaseModel):
    text: str

# The original GoRankResult becomes TestRankResult
# Core change: the "relevance_score" field is renamed to "score"
class TestRankResult(BaseModel):
    index: int
    document: DocumentInfo
    score: float  # <--- [key change] field renamed from relevance_score to score

# Final response body structure; its "results" list holds TestRankResult items
class TestFinalResponse(BaseModel):
    results: List[TestRankResult]

# --- Change end ---


# --- 2. Load the model (once, at service startup) ---
print("Loading the model, please wait...")
device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
print(f"Using device: {device}")
try:
    # Make sure this path is correct
    model_path = '/data1/home/lwx/work/Download/rerank_model_weight'
    tokenizer = AutoTokenizer.from_pretrained(model_path)
    model = AutoModelForSequenceClassification.from_pretrained(model_path)
    model.to(device)
    model.eval()
    print("Model loaded.")
except Exception as e:
    print(f"Failed to load the model: {e}")
    # In a test environment, consider exiting when the model fails to load, so a broken service does not keep running
    exit()

# --- 3. Create the FastAPI app ---
app = FastAPI(
    title="Reranker API (Test Version)",
    description="An API service that returns a 'score' field to test Go client compatibility",
    version="1.0.2"
)

# --- 4. Define the API endpoints ---
# --- Change start: point response_model at the new test response structure ---
@app.post("/rerank", response_model=TestFinalResponse) # <--- [key change] response_model is now TestFinalResponse
def rerank_endpoint(request: RerankRequest):
    # --- Change end ---

    pairs = [[request.query, doc] for doc in request.documents]

    with torch.no_grad():
        inputs = outputs = logits = None

        try:
            inputs = tokenizer(pairs, padding=True, truncation=True, return_tensors='pt', max_length=1024).to(device)
            outputs = model(**inputs, return_dict=True)
            logits = outputs.logits.view(-1, ).float()
            scores = torch.sigmoid(logits)
        finally:
            # Free GPU resources
            del inputs, outputs, logits
            gc.collect()

            if torch.cuda.is_available():
                torch.cuda.empty_cache()
            elif hasattr(torch, "mps") and torch.mps.is_available():
                torch.mps.empty_cache()


    # --- Change start: build the results with the test structure ---
    results = []
    for i, (text, score_val) in enumerate(zip(request.documents, scores)):
        
        # 1. Create the nested document object
        doc_info = DocumentInfo(text=text)
        
        # 2. Create the TestRankResult object
        #    Note the field names: index, document, score
        test_result = TestRankResult(
            index=i,
            document=doc_info,
            score=score_val.item()  # <--- [key change] assigned to the "score" field
        )
        results.append(test_result)

    # 3. Sort (the key also changes to score)
    sorted_results = sorted(results, key=lambda x: x.score, reverse=True)
    # --- Change end ---
    
    # Return a dict; FastAPI validates and serializes it against response_model (TestFinalResponse)
    # The resulting JSON is {"results": [{"index": ..., "document": ..., "score": ...}]}
    return {"results": sorted_results}

@app.get("/")
def read_root():
    return {"status": "Reranker API (Test Version) is running"}

# --- 5. Start the service ---
if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=8000)
