import re
from fastapi import FastAPI
from pydantic import BaseModel
import spacy

# Load spaCy NLP model, handle case if not downloaded
try:
    nlp = spacy.load("en_core_web_sm")
except OSError:
    import subprocess
    import sys
    print("Downloading en_core_web_sm...")
    subprocess.check_call([sys.executable, "-m", "spacy", "download", "en_core_web_sm"])
    nlp = spacy.load("en_core_web_sm")

app = FastAPI()

class ScrubRequest(BaseModel):
    text: str

class ScrubResponse(BaseModel):
    clean_text: str
    stats: dict

# Compiled regex patterns for fast initial pass
PATTERNS = {
    "EMAIL": re.compile(r'[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}'),
    "PHONE": re.compile(r'\b(?:\+?\d{1,3}[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b'),
    "CREDIT_CARD": re.compile(r'\b(?:\d[ -]*?){13,16}\b'),
    "SSN": re.compile(r'\b\d{3}-\d{2}-\d{4}\b')
}

def scrub_regex(text: str) -> tuple[str, dict]:
    stats = {"EMAIL": 0, "PHONE": 0, "CREDIT_CARD": 0, "SSN": 0}
    for entity_type, pattern in PATTERNS.items():
        matches = pattern.findall(text)
        if matches:
            stats[entity_type] += len(matches)
            text = pattern.sub(f"[REDACTED_{entity_type}]", text)
    return text, stats

def scrub_spacy(text: str, stats: dict) -> tuple[str, dict]:
    doc = nlp(text)
    redacted_text = text
    
    # Process entities in reverse order to not mess up indices when replacing
    entities = [(ent.start_char, ent.end_char, ent.label_) for ent in doc.ents 
                if ent.label_ in ["PERSON", "ORG", "GPE", "LOC", "MONEY"]]
    entities.sort(key=lambda x: x[0], reverse=True)
    
    for start, end, label in entities:
        # Check if already redacted by regex
        if "[REDACTED_" not in redacted_text[start:end]:
            redacted_text = redacted_text[:start] + f"[REDACTED_{label}]" + redacted_text[end:]
            stats[label] = stats.get(label, 0) + 1
            
    return redacted_text, stats

@app.post("/scrub", response_model=ScrubResponse)
async def scrub_pii(req: ScrubRequest):
    # Pass 1: Blazing fast Regex
    text_regex, stats = scrub_regex(req.text)
    
    # Pass 2: Context-aware spaCy NER
    final_text, final_stats = scrub_spacy(text_regex, stats)
    
    return ScrubResponse(clean_text=final_text, stats=final_stats)

@app.get("/health")
async def health():
    return {"status": "ok"}
