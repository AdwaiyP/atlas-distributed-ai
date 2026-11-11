from contextlib import asynccontextmanager
from datetime import datetime
from uuid import UUID

from fastapi import Depends, FastAPI, HTTPException
from pydantic import BaseModel, Field
from sqlalchemy import text
from sqlalchemy.orm import Session

from backend.app.core.database import (
    Base,
    SessionLocal,
    engine,
)
from backend.app.models.task import Task


@asynccontextmanager
async def lifespan(app: FastAPI):
    # Development-only table initialization.
    # Replace with Alembic migrations later.
    Base.metadata.create_all(bind=engine)
    yield


app = FastAPI(
    title="ATLAS Distributed AI Platform",
    version="0.2.0",
    lifespan=lifespan,
)


def get_db():
    with SessionLocal() as db:
        yield db


class TaskRequest(BaseModel):
    prompt: str = Field(
        min_length=1,
        max_length=4000
    )


class TaskResponse(BaseModel):
    task_id: UUID
    status: str
    created_at: datetime


@app.get("/health")
def health():
    return {
        "status": "healthy",
        "service": "atlas-api"
    }


@app.get("/ready")
def readiness(db: Session = Depends(get_db)):
    try:
        db.execute(text("SELECT 1"))
        return {"status": "ready"}
    except Exception:
        raise HTTPException(
            status_code=503,
            detail="Database unavailable"
        )


@app.post(
    "/api/v1/tasks",
    response_model=TaskResponse,
    status_code=202,
)
def create_task(
    request: TaskRequest,
    db: Session = Depends(get_db),
):
    task = Task(
        prompt=request.prompt,
        status="queued"
    )

    db.add(task)
    db.commit()
    db.refresh(task)

    return TaskResponse(
        task_id=task.id,
        status=task.status,
        created_at=task.created_at
    )


@app.get("/api/v1/tasks/{task_id}")
def get_task(
    task_id: UUID,
    db: Session = Depends(get_db),
):
    task = db.get(Task, task_id)

    if task is None:
        raise HTTPException(
            status_code=404,
            detail="Task not found"
        )

    return {
        "task_id": task.id,
        "status": task.status,
        "result": task.result,
        "created_at": task.created_at
    }