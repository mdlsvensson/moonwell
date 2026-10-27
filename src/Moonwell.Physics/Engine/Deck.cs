namespace Moonwell.Engine
{
    /// <summary>
    /// One placed bridge's deck in the world: its type's shape, heights above the ground every cell along and across it in
    /// the bridge's own frame (at facing 270 and scale 1, as Decks.txt holds it), turned and scaled about the place the game
    /// reports for the bridge, and standing on the ground beneath any water there, as the game sets it (Terrain.Height on
    /// dry ground; over water, the ground beneath that the build wrote, as the grid keeps only the water's surface). A
    /// point whose nearest cell has no deck is off it; on it, the height is blended between the four cells around the
    /// point where all four have a deck, else the nearest's.
    /// </summary>
    internal sealed class Deck
    {
        /// <summary>The shape's heights above the ground, row by row from the first (southmost at facing 270), each row west to east; Decks.None where there is no deck.</summary>
        private readonly float[] values;

        /// <summary>The cells along a row.</summary>
        private readonly int columns;

        /// <summary>The rows.</summary>
        private readonly int rows;

        /// <summary>The column of the bridge's place.</summary>
        private readonly int originColumn;

        /// <summary>The row of the bridge's place.</summary>
        private readonly int originRow;

        /// <summary>The bridge's place, as the game reports it: x.</summary>
        private readonly float x;

        /// <summary>The bridge's place, as the game reports it: y.</summary>
        private readonly float y;

        /// <summary>The cosine of the angle the shape is turned by.</summary>
        private readonly float cos;

        /// <summary>The sine of the angle the shape is turned by.</summary>
        private readonly float sin;

        /// <summary>The width of a cell in the world: the shape's cell times the scale.</summary>
        private readonly float cell;

        /// <summary>The bridge's scale, which scales the heights too.</summary>
        private readonly float scale;

        /// <summary>The ground's height beneath any water at the bridge's place, which the heights stand on.</summary>
        private readonly float ground;

        /// <summary>The bridge.</summary>
        internal readonly Destructable bridge;

        /// <summary>The box the deck lies in: its least x.</summary>
        internal readonly float minX;

        /// <summary>The box's least y.</summary>
        internal readonly float minY;

        /// <summary>The box's greatest x.</summary>
        internal readonly float maxX;

        /// <summary>The box's greatest y.</summary>
        internal readonly float maxY;

        /// <summary>Whether the deck counts as ground: until its bridge dies or leaves the game.</summary>
        internal bool living = true;

        /// <summary>
        /// Makes a bridge's deck from its shape, turned by an angle in degrees and scaled about the bridge's place, on the
        /// ground beneath any water there: Terrain.Height where no corner of its tile is under water, else the ground beneath
        /// the build wrote for it.
        /// </summary>
        internal Deck(Destructable bridge, float[] values, float cellSize, int columns, int rows, int originColumn, int originRow, float turn, float scale, float beneath)
        {
            this.bridge = bridge;
            this.values = values;
            this.columns = columns;
            this.rows = rows;
            this.originColumn = originColumn;
            this.originRow = originRow;
            x = bridge.X;
            y = bridge.Y;
            cos = Angles.Cos(turn);
            sin = Angles.Sin(turn);
            cell = cellSize * scale;
            this.scale = scale;
            ground = Terrain.Grid.IsWet(x, y) ? beneath : Terrain.Height(x, y);

            // The box: the four corners of the grid's outer cells, turned.
            var left = (-originColumn - 0.5f) * cell;
            var right = (columns - 1 - originColumn + 0.5f) * cell;
            var bottom = (-originRow - 0.5f) * cell;
            var top = (rows - 1 - originRow + 0.5f) * cell;
            var spanX = (cos < 0.0f ? -cos : cos) * (right - left) * 0.5f + (sin < 0.0f ? -sin : sin) * (top - bottom) * 0.5f;
            var spanY = (sin < 0.0f ? -sin : sin) * (right - left) * 0.5f + (cos < 0.0f ? -cos : cos) * (top - bottom) * 0.5f;
            var middleX = (left + right) * 0.5f;
            var middleY = (bottom + top) * 0.5f;
            var centreX = x + middleX * cos - middleY * sin;
            var centreY = y + middleX * sin + middleY * cos;
            minX = centreX - spanX;
            maxX = centreX + spanX;
            minY = centreY - spanY;
            maxY = centreY + spanY;
        }

        /// <summary>Gives the deck's height at a point; false when the point is off the deck.</summary>
        internal bool Height(float px, float py, out float height)
        {
            height = 0.0f;
            float u;
            float v;
            if (!Locate(px, py, out u, out v))
            {
                return false;
            }

            var near = values[(int)(v + 0.5f) * columns + (int)(u + 0.5f)];
            if (near == Decks.None)
            {
                return false;
            }

            float blended;
            float alongU;
            float alongV;
            height = ground + scale * (Blend(u, v, out blended, out alongU, out alongV) ? blended : near);
            return true;
        }

        /// <summary>Whether a point is on the deck, its nearest cell having one: Height's test, with no height blended.</summary>
        internal bool Covers(float px, float py)
        {
            float u;
            float v;
            return Locate(px, py, out u, out v) && values[(int)(v + 0.5f) * columns + (int)(u + 0.5f)] != Decks.None;
        }

        /// <summary>Gives the direction straight out of the deck at a point on it, of length 1: from the blend of its cells, or straight up at its edge, where fewer than four cells around the point have a deck.</summary>
        internal void Normal(float px, float py, out float nx, out float ny, out float nz)
        {
            nx = 0.0f;
            ny = 0.0f;
            nz = 1.0f;
            float u;
            float v;
            float blended;
            float alongU;
            float alongV;
            if (!Locate(px, py, out u, out v) || !Blend(u, v, out blended, out alongU, out alongV))
            {
                return;
            }

            // The change of the height per game unit along the bridge's frame, turned into the world's.
            var frameX = alongU * scale / cell;
            var frameY = alongV * scale / cell;
            var alongX = frameX * cos - frameY * sin;
            var alongY = frameX * sin + frameY * cos;
            var length = Trig.Sqrt(alongX * alongX + alongY * alongY + 1.0f);
            nx = -alongX / length;
            ny = -alongY / length;
            nz = 1.0f / length;
        }

        /// <summary>Gives where a point is in the shape, in cells from its first column and row; false when its nearest cell is outside the grid (or the point is NaN).</summary>
        private bool Locate(float px, float py, out float u, out float v)
        {
            var dx = px - x;
            var dy = py - y;
            u = (dx * cos + dy * sin) / cell + originColumn;
            v = (dy * cos - dx * sin) / cell + originRow;
            return u > -0.5f && u < columns - 0.5f && v > -0.5f && v < rows - 0.5f;
        }

        /// <summary>Gives the height blended between the four cells around a place in the shape, and its change per cell along a row and up the rows; false when one of them is outside the grid or has no deck.</summary>
        private bool Blend(float u, float v, out float value, out float alongU, out float alongV)
        {
            value = 0.0f;
            alongU = 0.0f;
            alongV = 0.0f;
            if (u < 0.0f || v < 0.0f)
            {
                return false;
            }

            var c = (int)u;
            var r = (int)v;
            if (c + 1 >= columns || r + 1 >= rows)
            {
                return false;
            }

            var at = r * columns + c;
            var h00 = values[at];
            var h10 = values[at + 1];
            var h01 = values[at + columns];
            var h11 = values[at + columns + 1];
            if (h00 == Decks.None || h10 == Decks.None || h01 == Decks.None || h11 == Decks.None)
            {
                return false;
            }

            var fu = u - c;
            var fv = v - r;
            value = (1.0f - fv) * ((1.0f - fu) * h00 + fu * h10) + fv * ((1.0f - fu) * h01 + fu * h11);
            alongU = (1.0f - fv) * (h10 - h00) + fv * (h11 - h01);
            alongV = (1.0f - fu) * (h01 - h00) + fu * (h11 - h10);
            return true;
        }
    }
}
