using System.Collections.Generic;

namespace Moonwell.Engine
{
    /// <summary>Square cells over the world's bounds, each keeping its entries in the order they came in. A cell's list is made when the first entry goes in it, so memory follows what is on the map and not its size. A place outside the world counts in the edge cell.</summary>
    internal sealed class Grid
    {
        /// <summary>The world's left edge.</summary>
        private readonly float left;

        /// <summary>The world's bottom edge.</summary>
        private readonly float bottom;

        /// <summary>The side of a cell.</summary>
        private readonly float side;

        /// <summary>How many cells make a row.</summary>
        private readonly int columns;

        /// <summary>How many rows of cells there are.</summary>
        private readonly int rows;

        /// <summary>The cells, indexed row * columns + column from the bottom-left; null until something goes in.</summary>
        private readonly List<List<Entry>?> cells = new List<List<Entry>?>();

        /// <summary>How many cell lists have been made.</summary>
        public int Made;

        /// <summary>Makes a grid over a world's bounds with cells of the given side; no cell has a list yet.</summary>
        public Grid(float minX, float minY, float maxX, float maxY, float cellSide)
        {
            left = minX;
            bottom = minY;
            side = cellSide;
            columns = CountOf(maxX - minX, cellSide);
            rows = CountOf(maxY - minY, cellSide);
            var total = columns * rows;
            for (var i = 0; i < total; i++)
            {
                cells.Add(null);
            }
        }

        /// <summary>Puts an entry in the cell its place is in.</summary>
        public void Add(Entry entry)
        {
            Attach(entry, IndexOf(entry.X, entry.Y));
        }

        /// <summary>Sets an entry's place. Its cell changes only when the index changes, so a move inside a cell leaves every list as it was. An entry in no cell gets its place and stays in none.</summary>
        public void Move(Entry entry, float x, float y)
        {
            entry.X = x;
            entry.Y = y;
            if (entry.Cell < 0)
            {
                return;
            }

            var index = IndexOf(x, y);
            if (index == entry.Cell)
            {
                return;
            }

            Shift(entry, index);
        }

        /// <summary>Moves an entry that is in a cell to the end of another cell's list; a unit read that finds it in a new cell calls it, having set its place.</summary>
        public void Shift(Entry entry, int index)
        {
            Detach(entry);
            Attach(entry, index);
        }

        /// <summary>Takes an entry out of its cell; an entry in none, one never added or already removed, is left alone.</summary>
        public void Remove(Entry entry)
        {
            if (entry.Cell >= 0)
            {
                Detach(entry);
            }
        }

        /// <summary>Adds to the list the entries of the cells that a box touches, each entry once: the cells row by row from the bottom-left, and a cell's entries in the order they came into it. A box past the world's edge takes in the edge cells and so what lies outside.</summary>
        public void Collect(float minX, float minY, float maxX, float maxY, List<Entry> into)
        {
            var firstColumn = ColumnOf(minX);
            var lastColumn = ColumnOf(maxX);
            var firstRow = RowOf(minY);
            var lastRow = RowOf(maxY);
            for (var row = firstRow; row <= lastRow; row++)
            {
                for (var column = firstColumn; column <= lastColumn; column++)
                {
                    // A cell not made yet holds null (System.null in Lua), so the cell is read through the List.
                    var cell = cells[row * columns + column];
                    if (cell == null)
                    {
                        continue;
                    }

                    var count = cell.Count;
                    for (var i = 0; i < count; i++)
                    {
                        Raw.Append(into, Raw.At(cell, i));
                    }
                }
            }
        }

        /// <summary>The list of a cell by its index; null while nothing has gone in it.</summary>
        public List<Entry>? CellList(int index)
        {
            return cells[index];
        }

        /// <summary>The index of the cell a place is in, with the edge cell for what lies outside. It is worked in one call, as a unit read asks it for every unit it reads.</summary>
        public int IndexOf(float x, float y)
        {
            var across = (x - left) / side;
            var column = across <= 0.0f ? 0 : across >= columns ? columns - 1 : Raw.Whole(across);
            var up = (y - bottom) / side;
            var row = up <= 0.0f ? 0 : up >= rows ? rows - 1 : Raw.Whole(up);
            return row * columns + column;
        }

#if DEBUG
        /// <summary>The column of a cell by its index, counted from the left, for the debug mode.</summary>
        public int ColumnOfCell(int index)
        {
            return index % columns;
        }

        /// <summary>The row of a cell by its index, counted from the bottom, for the debug mode.</summary>
        public int RowOfCell(int index)
        {
            return index / columns;
        }

        /// <summary>The column a position from left to right falls in, with the edge column for what lies outside, for the debug mode.</summary>
        public int ColumnAt(float x)
        {
            return ColumnOf(x);
        }

        /// <summary>The row a position from bottom to top falls in, with the edge row for what lies outside, for the debug mode.</summary>
        public int RowAt(float y)
        {
            return RowOf(y);
        }

        /// <summary>The left edge of a column, for the debug mode.</summary>
        public float ColumnLeft(int column)
        {
            return left + column * side;
        }

        /// <summary>The bottom edge of a row, for the debug mode.</summary>
        public float RowBottom(int row)
        {
            return bottom + row * side;
        }
#endif

        /// <summary>The column a position from left to right falls in, with the edge column for what lies outside.</summary>
        private int ColumnOf(float x)
        {
            return CellOf((x - left) / side, columns);
        }

        /// <summary>The row a position from bottom to top falls in, with the edge row for what lies outside.</summary>
        private int RowOf(float y)
        {
            return CellOf((y - bottom) / side, rows);
        }

        /// <summary>Puts an entry at the end of a cell's list, making the list if it is the first.</summary>
        private void Attach(Entry entry, int index)
        {
            var cell = cells[index];
            if (cell == null)
            {
                cell = new List<Entry>();
                cells[index] = cell;
                Made++;
            }

            cell.Add(entry);
            entry.Cell = index;
        }

        /// <summary>Takes an entry out of the list it is in, keeping the order of the rest.</summary>
        private void Detach(Entry entry)
        {
            cells[entry.Cell]!.Remove(entry);
            entry.Cell = -1;
        }

        /// <summary>The cell for a distance counted in cells from the edge, clamped to the cells there are. It is clamped before it is made a whole number, so a place far outside cannot overflow.</summary>
        private static int CellOf(float distance, int count)
        {
            if (distance <= 0.0f)
            {
                return 0;
            }

            if (distance >= count)
            {
                return count - 1;
            }

            return Raw.Whole(distance);
        }

        /// <summary>How many cells of a side cover a length, at least 1.</summary>
        private static int CountOf(float length, float cellSide)
        {
            var count = (int)(length / cellSide);
            if (count * cellSide < length)
            {
                count++;
            }

            return count < 1 ? 1 : count;
        }
    }
}
