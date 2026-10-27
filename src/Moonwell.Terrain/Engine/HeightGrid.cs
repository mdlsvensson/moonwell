using System;

namespace Moonwell.Engine
{
    /// <summary>
    /// The ground's corners, read from the text the build wrote: four characters a corner, row by row from the
    /// bottom left, and each corner a 24-bit number, <c>h &lt;&lt; 7 | level &lt;&lt; 3 | ramp &lt;&lt; 1 | water</c>, where
    /// <c>h</c> is the height times 20, plus 49152. The corners are unpacked a block of 16 by 16 at a time, the first
    /// time one of them is read, so a map pays only for the ground it looks at.
    /// </summary>
    internal sealed class HeightGrid
    {
        /// <summary>The corners across and up one block.</summary>
        private const int Side = 16;

        /// <summary>Side as a power of two: a corner's block is its column and row shifted down by it.</summary>
        private const int SideBits = 4;

        /// <summary>The width of a tile, between corners.</summary>
        private const float TileSize = 128f;

        /// <summary>The 17-bit number that stands for a height of 0.</summary>
        private const int ZeroHeight = 49152;

        /// <summary>How far Span's bounds are kept from a block's lowest and highest corner, and its box from the block's edges: far more than a blend's rounding (thousandths), so every height read in the box is strictly between the bounds.</summary>
        private const float Margin = 1f;

        /// <summary>The corners' text.</summary>
        private readonly string corners;

        /// <summary>The blocks across the grid, the last one maybe partial.</summary>
        private readonly int blocksAcross;

        /// <summary>The heights of each block, indexed by the block; null until it is unpacked.</summary>
        private readonly float[][] heights;

        /// <summary>The low 7 bits of each corner (level, ramp, water) of each block; null until it is unpacked.</summary>
        private readonly int[][] flags;

        /// <summary>The lowest corner of each block's tiles (its own corners and the next blocks' first column and row), set when it is unpacked.</summary>
        private readonly float[] lows;

        /// <summary>The highest corner of each block's tiles, as lows.</summary>
        private readonly float[] highs;

        /// <summary>The corners across.</summary>
        internal readonly int Columns;

        /// <summary>The corners up.</summary>
        internal readonly int Rows;

        /// <summary>The x of corner 0, 0.</summary>
        internal readonly float Left;

        /// <summary>The y of corner 0, 0.</summary>
        internal readonly float Bottom;

        /// <summary>The x of the last column of corners.</summary>
        internal readonly float Right;

        /// <summary>The y of the last row of corners.</summary>
        internal readonly float Top;

        /// <summary>The last corner across, in tiles from the first: a point is kept inside the grid up to it.</summary>
        private readonly float lastColumn;

        /// <summary>The last corner up, in tiles from the first.</summary>
        private readonly float lastRow;

        /// <summary>The column of the last tile's lower-left corner (0 for a grid one corner across).</summary>
        private readonly int lastTileColumn;

        /// <summary>The row of the last tile's lower-left corner (0 for a grid one corner up).</summary>
        private readonly int lastTileRow;

        /// <summary>How many blocks have been unpacked; the cases read it to see that a read unpacks one block, once.</summary>
        internal int Unpacked { get; private set; }

        /// <summary>Makes a grid from its text. Throws ArgumentException when the text is not four characters a corner.</summary>
        internal HeightGrid(int columns, int rows, float left, float bottom, string corners)
        {
            if (columns < 1 || rows < 1 || corners.Length != columns * rows * 4)
            {
                throw new ArgumentException("A grid of " + columns + " by " + rows + " corners is " + columns * rows * 4 + " characters, not " + corners.Length);
            }

            this.corners = corners;
            Columns = columns;
            Rows = rows;
            Left = left;
            Bottom = bottom;
            lastColumn = columns - 1;
            lastRow = rows - 1;
            lastTileColumn = Math.Max(columns - 2, 0);
            lastTileRow = Math.Max(rows - 2, 0);
            Right = left + (columns - 1) * TileSize;
            Top = bottom + (rows - 1) * TileSize;
            blocksAcross = (columns + Side - 1) / Side;
            var blocks = blocksAcross * ((rows + Side - 1) / Side);
            heights = new float[blocks][];
            flags = new int[blocks][];
            lows = new float[blocks];
            highs = new float[blocks];
        }

        /// <summary>The height of a corner; a corner outside the grid reads as the nearest one inside.</summary>
        internal float Z(int i, int j)
        {
            var block = Unpack(i, j);
            return heights[block][Slot(i, j)];
        }

        /// <summary>The cliff level of a corner, 0 and up; a corner outside the grid reads as the nearest one inside.</summary>
        internal int CliffLevel(int i, int j)
        {
            var block = Unpack(i, j);
            return (flags[block][Slot(i, j)] >> 3) & 15;
        }

        /// <summary>Whether a corner is on a ramp; a corner outside the grid reads as the nearest one inside.</summary>
        internal bool IsRamp(int i, int j)
        {
            var block = Unpack(i, j);
            return (flags[block][Slot(i, j)] & 2) != 0;
        }

        /// <summary>Whether a corner is under water; a corner outside the grid reads as the nearest one inside.</summary>
        internal bool IsWater(int i, int j)
        {
            var block = Unpack(i, j);
            return (flags[block][Slot(i, j)] & 1) != 0;
        }

        /// <summary>
        /// The ground's height at a point, which is first kept inside the grid: the four corners of its tile blended
        /// bilinearly, in every tile, cliff tiles included. Every read of the ground comes here, so it is written out in
        /// one: a tile whose corners are in one block reads them from its slots, and any other a corner at a time.
        /// </summary>
        internal float Height(float x, float y)
        {
            // Locate, written out: it is a call less.
            var u = (x - Left) / TileSize;
            if (!(u > 0f))
            {
                u = 0f;
            }
            else if (!(u < lastColumn))
            {
                u = lastColumn;
            }

            var v = (y - Bottom) / TileSize;
            if (!(v > 0f))
            {
                v = 0f;
            }
            else if (!(v < lastRow))
            {
                v = lastRow;
            }

            var i = Slots.Whole(u);
            if (i > lastTileColumn)
            {
                i = lastTileColumn;
            }

            var j = Slots.Whole(v);
            if (j > lastTileRow)
            {
                j = lastTileRow;
            }

            var fx = u - i;
            var fy = v - j;
            float z00;
            float z10;
            float z01;
            float z11;
            var across = i & (Side - 1);
            var up = j & (Side - 1);
            if (across != Side - 1 && up != Side - 1 && i + 1 < Columns && j + 1 < Rows)
            {
                // The four corners are inside the grid and in one block, the tile's corner's.
                var block = (j >> SideBits) * blocksAcross + (i >> SideBits);
                if (!Slots.Has(heights, block))
                {
                    Unpack(i, j);
                }

                var z = Slots.At(heights, block);
                var slot = up * Side + across;
                z00 = Slots.At(z, slot);
                z10 = Slots.At(z, slot + 1);
                z01 = Slots.At(z, slot + Side);
                z11 = Slots.At(z, slot + Side + 1);
            }
            else
            {
                // A tile on a block's edge, or in a grid one corner across or up, whose far corners are kept on its near ones.
                var right = i + 1 < Columns ? i + 1 : i;
                var top = j + 1 < Rows ? j + 1 : j;
                z00 = Corner(i, j);
                z10 = Corner(right, j);
                z01 = Corner(i, top);
                z11 = Corner(right, top);
            }

            return (1f - fy) * ((1f - fx) * z00 + fx * z10) + fy * ((1f - fx) * z01 + fx * z11);
        }

        /// <summary>
        /// Whether a corner of the tile that holds a point (kept inside the grid) is under water: there Height blends the
        /// water's surface, not the ground beneath it, which the grid does not keep.
        /// </summary>
        internal bool IsWet(float x, float y)
        {
            Locate(x, y, out var i, out var j, out _, out _);
            var right = i + 1 < Columns ? i + 1 : i;
            var top = j + 1 < Rows ? j + 1 : j;
            return IsWater(i, j) || IsWater(right, j) || IsWater(i, top) || IsWater(right, top);
        }

        /// <summary>
        /// Whether the tile that holds a point (kept inside the grid) is a cliff: on more than one cliff level, with no
        /// ramp corner.
        /// </summary>
        internal bool IsCliff(float x, float y)
        {
            Locate(x, y, out var i, out var j, out _, out _);
            return IsCliffTile(i, j);
        }

        /// <summary>
        /// The slope of the blended ground at a point, as the height's change along x and along y: the blend's
        /// derivatives, in every tile.
        /// </summary>
        internal void Gradient(float x, float y, out float alongX, out float alongY)
        {
            Locate(x, y, out var i, out var j, out var fx, out var fy);
            var right = i + 1 < Columns ? i + 1 : i;
            var top = j + 1 < Rows ? j + 1 : j;
            var z00 = Corner(i, j);
            var z10 = Corner(right, j);
            var z01 = Corner(i, top);
            var z11 = Corner(right, top);
            alongX = ((1f - fy) * (z10 - z00) + fy * (z11 - z01)) / TileSize;
            alongY = ((1f - fx) * (z01 - z00) + fx * (z11 - z10)) / TileSize;
        }

        /// <summary>The height of a corner inside the grid, as Z reads it, with no keeping inside: its block is unpacked if it was not.</summary>
        private float Corner(int i, int j)
        {
            var block = (j >> SideBits) * blocksAcross + (i >> SideBits);
            if (!Slots.Has(heights, block))
            {
                Unpack(i, j);
            }

            return Slots.At(Slots.At(heights, block), (j & (Side - 1)) * Side + (i & (Side - 1)));
        }

        /// <summary>
        /// The box of a point's block, the tiles whose lower-left corner is in the block, drawn in by Margin, and the
        /// bounds of every height read in it: below low and above high, each Margin past the block's tiles' lowest and
        /// highest corner. Any point in the box is inside the grid and its tile's corners are among those, so Height there
        /// blends corners between them. False, and no box, for a point outside the grid's tiles, in a grid one corner
        /// across or up, or in a block not unpacked yet; a read at the point unpacks it.
        /// </summary>
        internal bool Span(float x, float y, out float minX, out float minY, out float maxX, out float maxY, out float low, out float high)
        {
            minX = 0f;
            minY = 0f;
            maxX = -1f;
            maxY = -1f;
            low = 0f;
            high = 0f;
            var u = (x - Left) / TileSize;
            var v = (y - Bottom) / TileSize;
            if (!(u >= 0f && u < lastColumn && v >= 0f && v < lastRow) || Columns < 2 || Rows < 2)
            {
                return false;
            }

            var column = Slots.Whole(u) >> SideBits;
            var row = Slots.Whole(v) >> SideBits;
            var block = row * blocksAcross + column;
            if (!Slots.Has(heights, block))
            {
                return false;
            }

            minX = Left + (column << SideBits) * TileSize + Margin;
            maxX = Left + Math.Min((column + 1) << SideBits, Columns - 1) * TileSize - Margin;
            minY = Bottom + (row << SideBits) * TileSize + Margin;
            maxY = Bottom + Math.Min((row + 1) << SideBits, Rows - 1) * TileSize - Margin;
            low = Slots.At(lows, block) - Margin;
            high = Slots.At(highs, block) + Margin;
            return true;
        }

        /// <summary>Whether the tile whose lower-left corner is (i, j) has corners on more than one cliff level and no ramp corner.</summary>
        private bool IsCliffTile(int i, int j)
        {
            if (IsRamp(i, j) || IsRamp(i + 1, j) || IsRamp(i, j + 1) || IsRamp(i + 1, j + 1))
            {
                return false;
            }

            var level = CliffLevel(i, j);
            return CliffLevel(i + 1, j) != level || CliffLevel(i, j + 1) != level || CliffLevel(i + 1, j + 1) != level;
        }

        /// <summary>Finds the tile of a point kept inside the grid: its lower-left corner and where the point is in it, 0 to 1 across and up.</summary>
        private void Locate(float x, float y, out int i, out int j, out float fx, out float fy)
        {
            var u = Clamp((x - Left) / TileSize, lastColumn);
            var v = Clamp((y - Bottom) / TileSize, lastRow);
            i = Slots.Whole(u);
            if (i > lastTileColumn)
            {
                i = lastTileColumn;
            }

            j = Slots.Whole(v);
            if (j > lastTileRow)
            {
                j = lastTileRow;
            }

            fx = u - i;
            fy = v - j;
        }

        /// <summary>A position in tiles, kept from 0 to the last corner's. A NaN is kept as 0: it is neither above 0 nor below the last.</summary>
        private static float Clamp(float tiles, float last)
        {
            if (!(tiles > 0f))
            {
                return 0f;
            }

            return tiles < last ? tiles : last;
        }

        /// <summary>A corner's column or row, kept inside the grid.</summary>
        private static int Inside(int index, int count)
        {
            if (index < 0)
            {
                return 0;
            }

            return index >= count ? count - 1 : index;
        }

        /// <summary>The place of a corner in its block's arrays.</summary>
        private int Slot(int i, int j)
        {
            return (Inside(j, Rows) % Side) * Side + Inside(i, Columns) % Side;
        }

        /// <summary>Unpacks the block that holds a corner if that was not done, and gives the block's number.</summary>
        private int Unpack(int i, int j)
        {
            var column = Inside(i, Columns);
            var row = Inside(j, Rows);
            var block = (row / Side) * blocksAcross + column / Side;
            if (heights[block] == null)
            {
                Fill(block, column / Side * Side, row / Side * Side);
                Unpacked++;
            }

            return block;
        }

        /// <summary>Reads the corners of a block, whose first corner is at a column and a row, and the lowest and highest of them and of the next block's first column and row (Span's bounds).</summary>
        private void Fill(int block, int firstColumn, int firstRow)
        {
            var z = new float[Side * Side];
            var low = new int[Side * Side];
            var lowest = HeightOf(Packed(firstColumn, firstRow));
            var highest = lowest;
            // Written out, not through Packed and HeightOf: a block is 256 corners, and a call less each.
            for (var dj = 0; dj < Side && firstRow + dj < Rows; dj++)
            {
                for (var di = 0; di < Side && firstColumn + di < Columns; di++)
                {
                    var at = ((firstRow + dj) * Columns + firstColumn + di) * 4;
                    var value = (Value(corners[at]) << 18) | (Value(corners[at + 1]) << 12) | (Value(corners[at + 2]) << 6) | Value(corners[at + 3]);
                    var height = ((value >> 7) - ZeroHeight) / 20f;
                    z[dj * Side + di] = height;
                    low[dj * Side + di] = value & 127;
                    if (height < lowest)
                    {
                        lowest = height;
                    }
                    else if (height > highest)
                    {
                        highest = height;
                    }
                }
            }

            // The tiles on the block's far edges reach the next block's first column and row.
            for (var d = 0; d <= Side; d++)
            {
                var right = firstColumn + Side < Columns && firstRow + d < Rows ? HeightOf(Packed(firstColumn + Side, firstRow + d)) : lowest;
                var top = firstRow + Side < Rows && firstColumn + d < Columns ? HeightOf(Packed(firstColumn + d, firstRow + Side)) : lowest;
                lowest = Math.Min(lowest, Math.Min(right, top));
                highest = Math.Max(highest, Math.Max(right, top));
            }

            heights[block] = z;
            flags[block] = low;
            lows[block] = lowest;
            highs[block] = highest;
        }

        /// <summary>The 24-bit number of a corner inside the grid, from its four characters of the text.</summary>
        private int Packed(int column, int row)
        {
            var at = (row * Columns + column) * 4;
            return (Value(corners[at]) << 18) | (Value(corners[at + 1]) << 12) | (Value(corners[at + 2]) << 6) | Value(corners[at + 3]);
        }

        /// <summary>The height a corner's number stands for.</summary>
        private static float HeightOf(int packed)
        {
            return ((packed >> 7) - ZeroHeight) / 20f;
        }

        /// <summary>The number, 0 to 63, a character of the text stands for: A to Z, a to z, 0 to 9, then + and /.</summary>
        private static int Value(char character)
        {
            var code = (int)character;
            if (code >= 65 && code <= 90)
            {
                return code - 65;
            }

            if (code >= 97 && code <= 122)
            {
                return code - 71;
            }

            if (code >= 48 && code <= 57)
            {
                return code + 4;
            }

            if (code == 43)
            {
                return 62;
            }

            if (code == 47)
            {
                return 63;
            }

            throw new ArgumentException("The grid's text has a character (code " + code + ") that is not one of A-Z a-z 0-9 + /");
        }
    }
}
