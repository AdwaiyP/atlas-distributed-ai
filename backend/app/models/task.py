
import uuid
from datetime import datetime, timezone

from sqlalchemy import DateTime, Index, Integer, String, Text, text
from sqlalchemy.orm import Mapped, mapped_column

from backend.app.core.database import Base


class Task(Base):
    __tablename__ = "tasks"
   
    __table_args__ = (
        Index(
            "idx_tasks_claim",
            "status",
            "created_at",
        ),
        Index(
            "idx_tasks_lease",
            "lease_expires_at",
            postgresql_where=text("status = 'running'"),
        ),
    )


    id: Mapped[uuid.UUID] = mapped_column(
        primary_key=True,
        default=uuid.uuid4,
    )

    prompt: Mapped[str] = mapped_column(
        Text,
        nullable=False,
    )

    status: Mapped[str] = mapped_column(
        String(20),
        default="queued",
        nullable=False,
        index=True,
    )

    result: Mapped[str | None] = mapped_column(
        Text,
        nullable=True,
    )

    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True),
        default=lambda: datetime.now(timezone.utc),
        nullable=False,
    )

    attempts: Mapped[int] = mapped_column(
        Integer,
        default=0,
        server_default="0",
        nullable=False,
    )

    max_attempts: Mapped[int] = mapped_column(
        Integer,
        default=3,
        server_default="3",
        nullable=False,
    )

    lease_expires_at: Mapped[datetime | None] = mapped_column(
        DateTime(timezone=True),
        nullable=True,
    )

    worker_id: Mapped[str | None] = mapped_column(
        Text,
        nullable=True,
    )

