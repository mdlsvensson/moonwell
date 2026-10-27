using System;
using System.Collections.Generic;
using Moonwell.Engine;

namespace Moonwell.Testing
{
    /// <summary>Cases for the full audit's fixes to areas: the cells the game is given, and what the place check asks.</summary>
    public static class AuditAreaCases
    {
        /// <summary>Installs the package on a fake world with no World Editor regions and returns it.</summary>
        private static FakeAreaWorld Fresh()
        {
            var fake = new FakeAreaWorld();
            AreaSystem.Install(fake, new string[0], new float[0]);
            return fake;
        }

        /// <summary>The fake's regions cover points as the game's do (game facts, 10-25 and 10-26): a cell from the multiple of 32 below to the next, and a rect from the cell of its min corner to the cell of its max corner, so a max edge on the 32 grid takes the cell past it.</summary>
        public static void FakeCoversAsTheGame(FakeWorld world)
        {
            var fake = new FakeAreaWorld();
            var square = fake.NewRegion();
            fake.AddRect(square, 0f, 0f, 256f, 256f);
            var made = fake.Of(square);
            Expect.True(fake.Covers(made, 0f, 100f) && fake.Covers(made, 256f, 100f) && fake.Covers(made, 287.9f, 100f), "(0, 0)-(256, 256) covers 0, 256 and 287.9");
            Expect.True(!fake.Covers(made, -0.1f, 100f) && !fake.Covers(made, 288f, 100f), "and not -0.1 or 288");
            var odd = fake.NewRegion();
            fake.AddRect(odd, 100.5f, 100.5f, 300.5f, 300.5f);
            Expect.True(fake.Covers(fake.Of(odd), 96f, 319.9f) && !fake.Covers(fake.Of(odd), 95.9f, 200f) && !fake.Covers(fake.Of(odd), 200f, 320f), "(100.5, 100.5)-(300.5, 300.5) covers 96 to 320");
            var cells = fake.NewRegion();
            fake.AddCell(cells, 10f, 10f);
            fake.AddCell(cells, -10f, -10f);
            Expect.True(fake.Covers(fake.Of(cells), 0f, 31.9f) && !fake.Covers(fake.Of(cells), 32f, 10f), "the cell at (10, 10) covers 0 to 32");
            Expect.True(fake.Covers(fake.Of(cells), -32f, -0.1f) && !fake.Covers(fake.Of(cells), -32.1f, -10f), "the cell at (-10, -10) covers -32 to 0");
        }

        /// <summary>A unit walked into a region gets an enter event, and out of it a leave event, once each.</summary>
        public static void FakeWalks(FakeWorld world)
        {
            var fake = Fresh();
            var area = new LogArea(AreaShape.Rect(new Bounds(new Vector2(0f, 0f), new Vector2(100f, 100f))));
            Game.Instance.Add(area);
            world.Advance(0f);
            var unit = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            area.Labels[unit] = "a";
            fake.Walk(unit.Handle, 500f, 500f);
            fake.Walk(unit.Handle, 50f, 50f);
            fake.Walk(unit.Handle, 60f, 60f);
            fake.Walk(unit.Handle, 500f, 50f);
            Expect.Equal("entered a, left a", string.Join(", ", area.Log.ToArray()), "one enter and one leave");
        }
    }
}
