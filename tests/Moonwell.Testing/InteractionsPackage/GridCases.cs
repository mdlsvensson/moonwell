using System.Collections.Generic;
using Moonwell.Engine;

namespace Moonwell.Testing
{
    /// <summary>Cases for the interactions package's settings and grid, on a 2,048 by 2,048 world from (-1024, -1024) with cells of 256: 8 columns and 8 rows.</summary>
    public static class GridCases
    {
        /// <summary>Installs a fake world and returns it.</summary>
        private static FakeInteractionWorld Fresh()
        {
            var fake = new FakeInteractionWorld();
            Interactions.Install(fake);
            return fake;
        }

        /// <summary>An entry for a new actor, not yet in the grid.</summary>
        private static Entry EntryAt(int id, float x, float y)
        {
            return new Entry(id, new TestActor(), 4) { X = x, Y = y };
        }

        /// <summary>An entry goes in the cell its position is in, counted row by row from the bottom-left, and the cell lists it.</summary>
        public static void Cells(FakeWorld world)
        {
            Fresh();
            var grid = Interactions.Grid;
            var entry = EntryAt(1, 10.0f, 10.0f);
            grid.Add(entry);
            Expect.Equal(4 * 8 + 4, entry.Cell, "the cell of (10, 10)");
            var cell = grid.CellList(entry.Cell);
            Expect.True(cell != null && cell.Count == 1 && cell[0] == entry, "the cell lists the entry");
            var corner = EntryAt(2, -1024.0f, -1024.0f);
            grid.Add(corner);
            Expect.Equal(0, corner.Cell, "the bottom-left corner is cell 0");
            var top = EntryAt(3, 1023.5f, 1023.5f);
            grid.Add(top);
            Expect.Equal(63, top.Cell, "the top-right corner is cell 63");
            var above = EntryAt(4, 10.0f, 300.0f);
            grid.Add(above);
            Expect.Equal(5 * 8 + 4, above.Cell, "the next row up");
        }

        /// <summary>Moving inside a cell leaves the cell's list as it was; moving across cells takes the entry out of one list and into the other.</summary>
        public static void Move(FakeWorld world)
        {
            Fresh();
            var grid = Interactions.Grid;
            var entry = EntryAt(1, 10.0f, 10.0f);
            grid.Add(entry);
            var first = grid.CellList(36);
            grid.Move(entry, 20.0f, 30.0f);
            Expect.Equal(36, entry.Cell, "still cell 36");
            Expect.Near(20.0f, entry.X, "X is set");
            Expect.Near(30.0f, entry.Y, "Y is set");
            Expect.Same(first, grid.CellList(36), "the same list");
            Expect.Equal(1, first!.Count, "one entry in it");
            grid.Move(entry, 300.0f, 10.0f);
            Expect.Equal(37, entry.Cell, "now cell 37");
            Expect.Equal(0, first.Count, "the old cell is empty");
            var second = grid.CellList(37);
            Expect.True(second != null && second.Count == 1 && second[0] == entry, "the new cell lists the entry");
            grid.Move(entry, 10.0f, 300.0f);
            Expect.Equal(44, entry.Cell, "now cell 44");
            Expect.Equal(0, second!.Count, "cell 37 is empty");
        }

        /// <summary>Collect gives each entry once, cells row by row from the bottom-left and the entries of a cell in the order they came in; it adds to the list it is given.</summary>
        public static void Collect(FakeWorld world)
        {
            Fresh();
            var grid = Interactions.Grid;
            var right = EntryAt(1, 300.0f, 10.0f);
            var left = EntryAt(2, 10.0f, 10.0f);
            var also = EntryAt(3, 20.0f, 20.0f);
            grid.Add(right);
            grid.Add(left);
            grid.Add(also);
            var found = new List<Entry>();
            grid.Collect(-1024.0f, -1024.0f, 1024.0f, 1024.0f, found);
            Expect.Equal(3, found.Count, "three entries");
            Expect.True(found[0] == left && found[1] == also && found[2] == right, "cell 36 in the order it came in, then cell 37");
            found.Clear();
            grid.Collect(0.0f, 0.0f, 100.0f, 100.0f, found);
            Expect.Equal(2, found.Count, "two entries in the cell of the box");
            Expect.True(found[0] == left && found[1] == also, "left, then also");
            var row = EntryAt(4, -900.0f, 300.0f);
            grid.Add(row);
            found.Clear();
            grid.Collect(-1024.0f, -1024.0f, 1024.0f, 1024.0f, found);
            Expect.Equal(4, found.Count, "four entries");
            Expect.True(found[2] == right && found[3] == row, "a row further up comes after");
            grid.Collect(0.0f, 0.0f, 100.0f, 100.0f, found);
            Expect.Equal(6, found.Count, "Collect adds to the list");
            found.Clear();
            grid.Collect(600.0f, 600.0f, 700.0f, 700.0f, found);
            Expect.Equal(0, found.Count, "a box over empty cells finds nothing");
        }

        /// <summary>A position outside the world counts in the edge cell, and so does a box that reaches past the world.</summary>
        public static void Edge(FakeWorld world)
        {
            Fresh();
            var grid = Interactions.Grid;
            var east = EntryAt(1, 5000.0f, 0.0f);
            var west = EntryAt(2, -5000.0f, 0.0f);
            var north = EntryAt(3, 0.0f, 5000.0f);
            var south = EntryAt(4, 0.0f, -5000.0f);
            var far = EntryAt(5, 5000.0f, 5000.0f);
            grid.Add(east);
            grid.Add(west);
            grid.Add(north);
            grid.Add(south);
            grid.Add(far);
            Expect.Equal(4 * 8 + 7, east.Cell, "(5000, 0) is in the right-hand column");
            Expect.Equal(4 * 8, west.Cell, "(-5000, 0) is in the left-hand column");
            Expect.Equal(7 * 8 + 4, north.Cell, "(0, 5000) is in the top row");
            Expect.Equal(4, south.Cell, "(0, -5000) is in the bottom row");
            Expect.Equal(63, far.Cell, "(5000, 5000) is the top-right cell");
            grid.Move(far, -5000.0f, -5000.0f);
            Expect.Equal(0, far.Cell, "moved past the other corner");
            var found = new List<Entry>();
            grid.Collect(4000.0f, -100.0f, 6000.0f, 100.0f, found);
            Expect.Equal(1, found.Count, "a box past the edge finds what is outside");
            Expect.True(found[0] == east, "it is the entry in the edge cell");
        }

        /// <summary>A cell is made when an entry first goes in it, and the world's bounds are read once.</summary>
        public static void Lazy(FakeWorld world)
        {
            var fake = Fresh();
            var grid = Interactions.Grid;
            Expect.Equal(0, grid.Made, "an empty grid has made no cell");
            var found = new List<Entry>();
            grid.Collect(-1024.0f, -1024.0f, 1024.0f, 1024.0f, found);
            Expect.Equal(0, grid.Made, "looking makes none");
            Expect.True(grid.CellList(36) == null, "there is no list for cell 36");
            var entry = EntryAt(1, 10.0f, 10.0f);
            grid.Add(entry);
            Expect.Equal(1, grid.Made, "one cell for the first entry");
            grid.Add(EntryAt(2, 20.0f, 20.0f));
            Expect.Equal(1, grid.Made, "the same cell for another");
            grid.Move(entry, 300.0f, 10.0f);
            Expect.Equal(2, grid.Made, "a cell for the move");
            Expect.Same(grid, Interactions.Grid, "the grid is one");
            Expect.Equal(1, fake.BoundsReads, "the bounds are read once");
        }

        /// <summary>Remove takes the entry out of its cell, and an entry that was never added, or is already out, is left alone.</summary>
        public static void Remove(FakeWorld world)
        {
            Fresh();
            var grid = Interactions.Grid;
            var first = EntryAt(1, 10.0f, 10.0f);
            var second = EntryAt(2, 20.0f, 20.0f);
            Expect.Equal(-1, first.Cell, "an entry starts in no cell");
            grid.Remove(first);
            Expect.Equal(-1, first.Cell, "removing one that was never added changes nothing");
            grid.Add(first);
            grid.Add(second);
            grid.Remove(first);
            Expect.Equal(-1, first.Cell, "removed");
            var cell = grid.CellList(36);
            Expect.True(cell != null && cell.Count == 1 && cell[0] == second, "the other stays");
            grid.Remove(first);
            Expect.Equal(1, cell!.Count, "removing again changes nothing");
            var found = new List<Entry>();
            grid.Collect(-1024.0f, -1024.0f, 1024.0f, 1024.0f, found);
            Expect.Equal(1, found.Count, "Collect no longer finds it");
        }

        /// <summary>The settings start at their defaults, and are read when the grid is made.</summary>
        public static void Settings(FakeWorld world)
        {
            Fresh();
            Expect.Near(256.0f, Interactions.CellSize, "CellSize");
            Expect.Near(0.1f, Interactions.CellCheck, "CellCheck");
            Expect.Near(64.0f, Interactions.Slack, "Slack");
            Expect.Near(0.03125f, Interactions.Step, "Step");
            Interactions.CellSize = 512.0f;
            try
            {
                var entry = EntryAt(1, 10.0f, 10.0f);
                Interactions.Grid.Add(entry);
                Expect.Equal(2 * 4 + 2, entry.Cell, "with cells of 512 the world is 4 by 4");
            }
            finally
            {
                Fresh();
                Interactions.CellSize = 256.0f;
            }

            Expect.Near(256.0f, Interactions.CellSize, "CellSize is back");
        }

        /// <summary>A setting changed after the grid exists is an error in a development build, and changes nothing.</summary>
        public static void SettingsLocked(FakeWorld world)
        {
            Fresh();
            var grid = Interactions.Grid;
#if DEBUG
            Expect.Throws<MoonwellException>(() => Interactions.CellSize = 512.0f, "CellSize after the grid");
            Expect.Throws<MoonwellException>(() => Interactions.CellCheck = 0.2f, "CellCheck after the grid");
            Expect.Throws<MoonwellException>(() => Interactions.Slack = 32.0f, "Slack after the grid");
            Expect.Throws<MoonwellException>(() => Interactions.Step = 0.02f, "Step after the grid");
            Expect.Near(256.0f, Interactions.CellSize, "CellSize is as it was");
#endif
            Expect.Same(grid, Interactions.Grid, "the grid is the same");
        }

        /// <summary>After the runtime is reset the package is not installed, and once installed again its next grid is a new one, with the settings free to change.</summary>
        public static void Reset(FakeWorld world)
        {
            Fresh();
            var first = Interactions.Grid;
            first.Add(EntryAt(1, 10.0f, 10.0f));
            FakeWorld.Install();
            var fake = Fresh();
            var second = Interactions.Grid;
            Expect.True(!object.ReferenceEquals(first, second), "a new grid");
            Expect.Equal(0, second.Made, "an empty one");
            Expect.Equal(1, fake.BoundsReads, "which read the bounds of the new world");
            FakeWorld.Install();
            Interactions.CellSize = 256.0f;
            Expect.Near(256.0f, Interactions.CellSize, "a setting can be set again after a reset");
        }
    }
}
