"""Create the v2 persistence schema.

The metadata-defined schema uses PostGIS geometry with SRID 4326 on PostgreSQL.
"""
from alembic import op
from sthira_v2.persistence.models import Base

revision = "20260911_01"
down_revision = None
branch_labels = None
depends_on = None

def upgrade() -> None:
    bind = op.get_bind()
    if bind.dialect.name == "postgresql":
        op.execute("CREATE EXTENSION IF NOT EXISTS postgis")
    Base.metadata.create_all(bind=bind)

def downgrade() -> None:
    Base.metadata.drop_all(bind=op.get_bind())
