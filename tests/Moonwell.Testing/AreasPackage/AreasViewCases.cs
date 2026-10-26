using Moonwell.Engine;
using Moonwell.Hosting;

namespace Moonwell.Testing
{
#if DEBUG
    /// <summary>An area of a shape with no reactions, named for the panel's lines.</summary>
    public sealed class Arena : Area
    {
        /// <summary>Makes an area of a shape.</summary>
        public Arena(AreaShape shape)
        {
            Shape = shape;
        }
    }

    /// <summary>A second kind of area with no reactions, so the panel's lines show the class's name.</summary>
    public sealed class Pit : Area
    {
        /// <summary>Makes an area of a shape.</summary>
        public Pit(AreaShape shape)
        {
            Shape = shape;
        }
    }

    /// <summary>Cases for the areas package's view in the debug mode: the lines of the areas a selected unit is in, and their outlines.</summary>
    public static class AreasViewCases
    {
        /// <summary>A rect from (x, y) to (x + size, y + size).</summary>
        private static AreaShape Square(float x, float y, float size)
        {
            return AreaShape.Rect(new Bounds(new Vector2(x, y), new Vector2(x + size, y + size)));
        }

        /// <summary>Makes a footman of the game at a place and takes it in.</summary>
        private static Unit UnitAt(FakeWorld world, float x, float y)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            world.MoveUnit(handle, x, y);
            return Unit.Of(handle)!;
        }

        /// <summary>Installs a fake areas world and the fake debug world, and turns the mode on with -debug.</summary>
        private static FakeDebugWorld On(FakeWorld world, out FakeAreaWorld areas)
        {
            areas = new FakeAreaWorld();
            AreaSystem.Install(areas, new string[0], new float[0]);
            TestLog.Lines.Clear();
            var debug = FakeDebugWorld.Install(world);
            debug.Chat(0, "-debug");
            return debug;
        }

        /// <summary>Adds an area to the game and lets its first, 0 second timer run.</summary>
        private static Area Added(FakeWorld world, Area area)
        {
            Game.Instance.Add(area);
            world.Advance(0f);
            return area;
        }

        /// <summary>A unit in two of three areas: its panel has a line for each, in the order the areas were added, with the seconds since it entered, and none for the area it is not in.</summary>
        public static void Lines(FakeWorld world)
        {
            var debug = On(world, out var areas);
            Added(world, new Arena(Square(0f, 0f, 256f)));
            Added(world, new Pit(Square(1000f, 1000f, 256f)));
            Added(world, new Pit(Square(0f, 0f, 512f)));
            var unit = UnitAt(world, 10.0f, 10.0f);
            areas.Enter(areas.Regions[2], unit.Handle);
            debug.Advance(1.0f);
            areas.Enter(areas.Regions[0], unit.Handle);
            debug.Advance(2.5f);
            DebugMode.Select(unit);
            var lines = debug.PanelLines();
            Expect.Equal(4, lines.Count, "the panel's lines");
            Expect.Equal("in Arena since 2.5 s", lines[2], "the first area added, entered later");
            Expect.Equal("in Pit since 3.5 s", lines[3], "the third area added, entered first");
        }

        /// <summary>A rect area outlines in 4 lines, a circle in 24, a shape of a rect and a circle in both; a polygon by its corners. All in Neutral, and one area at a time, as each unit is in one.</summary>
        public static void Outlines(FakeWorld world)
        {
            var debug = On(world, out var areas);
            Added(world, new Arena(Square(0f, 0f, 256f)));
            Added(world, new Pit(AreaShape.Circle(new Vector2(1000f, 1000f), 100f)));
            Added(world, new Arena(AreaShape.Of(Square(-1000f, 0f, 128f), AreaShape.Circle(new Vector2(-1000f, 500f), 64f))));
            Added(world, new Pit(AreaShape.Polygon(new Polygon(new Vector2(2000f, 0f), new Vector2(2200f, 0f), new Vector2(2100f, 150f)))));
            var inRect = UnitAt(world, 10.0f, 10.0f);
            var inCircle = UnitAt(world, 1000.0f, 1000.0f);
            var inBoth = UnitAt(world, -1000.0f, 500.0f);
            var inPolygon = UnitAt(world, 2100.0f, 50.0f);
            areas.Enter(areas.Regions[0], inRect.Handle);
            areas.Enter(areas.Regions[1], inCircle.Handle);
            areas.Enter(areas.Regions[2], inBoth.Handle);
            areas.Enter(areas.Regions[3], inPolygon.Handle);

            DebugMode.Select(inRect);
            Expect.Equal(4, debug.Lines.Count, "a rect's lines");
            Expect.Equal(1, debug.Markers.Count, "the marker, core's");
            for (var i = 0; i < 4; i++)
            {
                Expect.Equal(DebugDraw.Neutral, debug.LineStyles[i], "the style of rect line " + i);
            }

            Expect.Near(0.0f, debug.LineX1s[0], "the rect's first line starts at its left");
            Expect.Near(0.0f, debug.LineY1s[0], "the rect's first line starts at its bottom");
            Expect.Near(256.0f, debug.LineX2s[0], "the rect's first line ends at its right");
            Expect.Near(0.0f, debug.LineY2s[0], "the rect's first line is level");
            Expect.Near(256.0f, debug.LineY2s[1], "the rect's second line ends at its top");
            Expect.Near(0.0f, debug.LineX2s[3], "the rect's last line ends at its left");
            Expect.Near(0.0f, debug.LineY2s[3], "the rect's last line ends at its bottom");

            DebugMode.Select(inCircle);
            Expect.Equal(24, debug.Lines.Count, "a circle's lines");
            Expect.Near(1100.0f, debug.LineX1s[0], "the circle starts at its radius");
            Expect.Near(1000.0f, debug.LineY1s[0], "the circle starts on its centre's row");
            for (var i = 0; i < 24; i++)
            {
                Expect.Equal(DebugDraw.Neutral, debug.LineStyles[i], "the style of circle line " + i);
            }

            DebugMode.Select(inBoth);
            Expect.Equal(4 + 24, debug.Lines.Count, "the lines of a rect and a circle");
            Expect.Near(-1000.0f, debug.LineX1s[0], "the first part, the rect, comes first");
            Expect.Near(-936.0f, debug.LineX1s[4], "the second part, the circle, starts at its radius");
            Expect.Near(500.0f, debug.LineY1s[4], "the second part starts on its centre's row");

            DebugMode.Select(inPolygon);
            Expect.Equal(3, debug.Lines.Count, "a triangle's lines");
            Expect.Near(2000.0f, debug.LineX1s[0], "the first edge starts at the first corner");
            Expect.Near(2200.0f, debug.LineX2s[0], "the first edge ends at the second corner");
            Expect.Near(2100.0f, debug.LineX1s[2], "the closing edge starts at the last corner");
            Expect.Near(2000.0f, debug.LineX2s[2], "the closing edge ends at the first corner");
        }

        /// <summary>A unit in no area has no line and no outline; neither has a unit once it leaves the one it was in.</summary>
        public static void NotInAny(FakeWorld world)
        {
            var debug = On(world, out var areas);
            Added(world, new Arena(Square(0f, 0f, 256f)));
            var outside = UnitAt(world, 900.0f, 900.0f);
            var inside = UnitAt(world, 10.0f, 10.0f);
            areas.Enter(areas.Regions[0], inside.Handle);
            DebugMode.Select(outside);
            Expect.Equal(2, debug.PanelLines().Count, "the panel's lines for a unit in no area");
            Expect.Equal(0, debug.Lines.Count, "lines for a unit in no area");
            Expect.Equal(1, debug.Markers.Count, "the marker, core's");
            DebugMode.Select(inside);
            Expect.Equal(3, debug.PanelLines().Count, "the panel's lines for a unit inside");
            Expect.Equal(4, debug.Lines.Count, "lines for a unit inside");
            areas.Leave(areas.Regions[0], inside.Handle);
            debug.Advance(0.25f);
            Expect.Equal(2, debug.PanelLines().Count, "the panel's lines once it left");
            Expect.Equal(0, debug.Lines.Count, "lines once it left");
        }

        /// <summary>The view is added once, when the package makes its tracker, however many areas are added, and again after a reset.</summary>
        public static void Registered(FakeWorld world)
        {
            var areas = new FakeAreaWorld();
            AreaSystem.Install(areas, new string[0], new float[0]);
            Expect.Equal(0, DebugViews.All.Count, "views before an area is added");
            Added(world, new Arena(Square(0f, 0f, 256f)));
            Added(world, new Pit(Square(300f, 0f, 256f)));
            Expect.Equal(1, DebugViews.All.Count, "views after two areas");
            Runtime.Reset(world);
            Expect.Equal(0, DebugViews.All.Count, "views after a reset");
            AreaSystem.Install(new FakeAreaWorld(), new string[0], new float[0]);
            Added(world, new Arena(Square(0f, 0f, 256f)));
            Expect.Equal(1, DebugViews.All.Count, "views after a reset and an area");
        }
    }
#endif
}
