from fastapi import FastAPI

app = FastAPI()

@app.get("/")
def root():
    return {"status": "ok", "message": "Snowden_system ML Space is running"}

@app.get("/health")
def health():
    return {"status": "healthy"}
