using System;
using Moonwell.Hosting;

namespace Moonwell.Engine
{
    /// <summary>
    /// What the game says units can do on the ground, kept a cell of 32 by 32 at a time from the grid's first corner. A
    /// cell is read at its centre, four calls to the world, into one number: walkable 1, flyable 2, buildable 4, water
    /// 8. The cells are read a block of 16 by 16 at a time, the first time one of them is asked for, so a map pays only
    /// for the ground it looks at; <c>Refresh</c> drops blocks so the next ask reads them again.
    /// </summary>
    internal sealed class PathingGrid
    {
        /// <summary>The flag of a cell where units can walk.</summary>
        internal const int Walkable = 1;

        /// <summary>The flag of a cell where flying units can fly.</summary>
        internal const int Flyable = 2;

        /// <summary>The flag of a cell where a building can be built.</summary>
        internal const int Buildable = 4;

        /// <summary>The flag of a cell where ships can float: water.</summary>
        internal const int Water = 8;

        /// <summary>The cells across and up one block.</summary>
        private const int Side = 16;

        /// <summary>The width of a cell, which is also how finely the game's pathing is kept.</summary>
        private const float CellSize = 32f;

        /// <summary>The world the cells are read from.</summary>
        private readonly ITerrainWorld world;

        /// <summary>The cells across.</summary>
        private readonly int columns;

        /// <summary>The cells up.</summary>
        private readonly int rows;

        /// <summary>The x of the first cell's lower left.</summary>
        private readonly float left;

        /// <summary>The y of the first cell's lower left.</summary>
        private readonly float bottom;

        /// <summary>The blocks across the grid, the last one maybe partial.</summary>
        private readonly int blocksAcross;

        /// <summary>The flags of each block, indexed by the block; null until it is read.</summary>
        private readonly int[]?[] blocks;

        /// <summary>Makes a grid over the area of a height grid of this many corners (a tile is 4 cells), reading nothing yet.</summary>
        internal PathingGrid(ITerrainWorld world, int cornersAcross, int cornersUp, float left, float bottom)
        {
            this.world = world;
            this.left = left;
            this.bottom = bottom;
            columns = Math.Max(cornersAcross - 1, 1) * 4;
            rows = Math.Max(cornersUp - 1, 1) * 4;
            blocksAcross = (columns + Side - 1) / Side;
            blocks = new int[blocksAcross * ((rows + Side - 1) / Side)][];
        }

        /// <summary>The flags of the cell that holds a point, which is first kept inside the grid.</summary>
        internal int Flags(float x, float y)
        {
            var column = Cell(x, left, columns);
            var row = Cell(y, bottom, rows);
            var block = (row / Side) * blocksAcross + column / Side;
            var flags = blocks[block];
            if (flags == null)
            {
                flags = Read(column / Side * Side, row / Side * Side);
                blocks[block] = flags;
            }

            return flags[(row % Side) * Side + column % Side];
        }

        /// <summary>Drops every block an area touches (the area kept inside the grid), so the next read of one reads it again.</summary>
        internal void Refresh(Bounds area)
        {
            var firstColumn = Cell(area.Min.X, left, columns) / Side;
            var lastColumn = Cell(area.Max.X, left, columns) / Side;
            var firstRow = Cell(area.Min.Y, bottom, rows) / Side;
            var lastRow = Cell(area.Max.Y, bottom, rows) / Side;
            for (var row = firstRow; row <= lastRow; row++)
            {
                for (var column = firstColumn; column <= lastColumn; column++)
                {
                    blocks[row * blocksAcross + column] = null;
                }
            }
        }

        /// <summary>The column or row of the cell that holds a coordinate, kept from the first to the last. It is kept before it is floored, as a cast of a negative number rounds the wrong way.</summary>
        private static int Cell(float coordinate, float origin, int count)
        {
            var cells = (coordinate - origin) / CellSize;
            if (cells < 0f)
            {
                cells = 0f;
            }

            if (cells > count - 1)
            {
                cells = count - 1;
            }

            return (int)MathF.Floor(cells);
        }

        /// <summary>Reads the cells of a block, whose first cell is at a column and a row; the cells past the grid's edge are not read.</summary>
        private int[] Read(int firstColumn, int firstRow)
        {
            var flags = new int[Side * Side];
            for (var dj = 0; dj < Side && firstRow + dj < rows; dj++)
            {
                var y = bottom + (firstRow + dj) * CellSize + CellSize / 2f;
                for (var di = 0; di < Side && firstColumn + di < columns; di++)
                {
                    var x = left + (firstColumn + di) * CellSize + CellSize / 2f;
                    var cell = 0;
                    if (world.IsWalkable(x, y))
                    {
                        cell |= Walkable;
                    }

                    if (world.IsFlyable(x, y))
                    {
                        cell |= Flyable;
                    }

                    if (world.IsBuildable(x, y))
                    {
                        cell |= Buildable;
                    }

                    if (world.IsFloatable(x, y))
                    {
                        cell |= Water;
                    }

                    flags[dj * Side + di] = cell;
                }
            }

            return flags;
        }
    }
}
