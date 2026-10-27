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

        /// <summary>The width of a tile, between corners.</summary>
        private const float TileSize = 128f;

        /// <summary>The 17-bit number that stands for a height of 0.</summary>
        private const int ZeroHeight = 49152;

        /// <summary>The corners' text.</summary>
        private readonly string corners;

        /// <summary>The blocks across the grid, the last one maybe partial.</summary>
        private readonly int blocksAcross;

        /// <summary>The heights of each block, indexed by the block; null until it is unpacked.</summary>
        private readonly float[][] heights;

        /// <summary>The low 7 bits of each corner (level, ramp, water) of each block; null until it is unpacked.</summary>
        private readonly int[][] flags;

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
            Right = left + (columns - 1) * TileSize;
            Top = bottom + (rows - 1) * TileSize;
            blocksAcross = (columns + Side - 1) / Side;
            var blocks = blocksAcross * ((rows + Side - 1) / Side);
            heights = new float[blocks][];
            flags = new int[blocks][];
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
        /// bilinearly, in every tile, cliff tiles included.
        /// </summary>
        internal float Height(float x, float y)
        {
            Locate(x, y, out var i, out var j, out var fx, out var fy);
            var z00 = Z(i, j);
            var z10 = Z(i + 1, j);
            var z01 = Z(i, j + 1);
            var z11 = Z(i + 1, j + 1);
            return (1f - fy) * ((1f - fx) * z00 + fx * z10) + fy * ((1f - fx) * z01 + fx * z11);
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
            var z00 = Z(i, j);
            var z10 = Z(i + 1, j);
            var z01 = Z(i, j + 1);
            var z11 = Z(i + 1, j + 1);
            alongX = ((1f - fy) * (z10 - z00) + fy * (z11 - z01)) / TileSize;
            alongY = ((1f - fx) * (z01 - z00) + fx * (z11 - z10)) / TileSize;
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
            var u = Clamp((x - Left) / TileSize, Columns - 1);
            var v = Clamp((y - Bottom) / TileSize, Rows - 1);
            i = Math.Min((int)MathF.Floor(u), Math.Max(Columns - 2, 0));
            j = Math.Min((int)MathF.Floor(v), Math.Max(Rows - 2, 0));
            fx = u - i;
            fy = v - j;
        }

        /// <summary>A position in tiles, kept from 0 to the last corner's. A NaN is kept as 0: it is neither above 0 nor below the last.</summary>
        private static float Clamp(float tiles, int last)
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

        /// <summary>Reads the corners of a block, whose first corner is at a column and a row.</summary>
        private void Fill(int block, int firstColumn, int firstRow)
        {
            var z = new float[Side * Side];
            var low = new int[Side * Side];
            for (var dj = 0; dj < Side && firstRow + dj < Rows; dj++)
            {
                for (var di = 0; di < Side && firstColumn + di < Columns; di++)
                {
                    var at = ((firstRow + dj) * Columns + firstColumn + di) * 4;
                    var value = (Value(corners[at]) << 18) | (Value(corners[at + 1]) << 12) | (Value(corners[at + 2]) << 6) | Value(corners[at + 3]);
                    z[dj * Side + di] = ((value >> 7) - ZeroHeight) / 20f;
                    low[dj * Side + di] = value & 127;
                }
            }

            heights[block] = z;
            flags[block] = low;
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
