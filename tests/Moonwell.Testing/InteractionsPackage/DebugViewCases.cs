using Moonwell.Engine;
using Moonwell.Hosting;

namespace Moonwell.Testing
{
#if DEBUG
    /// <summary>Cases for the interactions package's view in the debug mode: what a click finds, the lines of a holder and of an other, the ranges, cells and pair lines drawn, a selected gizmo that ends, the statistics, and the step length.</summary>
    public static class DebugViewCases
    {
        /// <summary>One step of the cycle on the default settings.</summary>
        private const float Step = 1.0f / 32.0f;

        /// <summary>Makes footmen (hfoo) Footman objects.</summary>
        private static void RegisterFootman()
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("hfoo"), () => new Footman());
        }

        /// <summary>A footman taken in at a place; RegisterFootman must have run.</summary>
        private static Footman FootmanAt(FakeWorld world, float x, float y)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            world.PlaceUnit(handle, x, y);
            return (Footman)Unit.Of(handle)!;
        }

        /// <summary>An item of the game at a place, on the world's list of items, taken in.</summary>
        private static Item ItemAt(FakeWorld world, float x, float y)
        {
            var handle = world.NewItem(ObjectTypes.FourCC("ofoo"));
            world.MoveItem(handle, x, y);
            world.PlacedItems.Add(handle);
            return Item.Of(handle)!;
        }

        /// <summary>Installs a fake interactions world and the fake debug world, and turns the mode on with -debug.</summary>
        private static FakeDebugWorld On(FakeWorld world)
        {
            Interactions.Install(new FakeInteractionWorld());
            TestLog.Lines.Clear();
            var debug = FakeDebugWorld.Install(world);
            debug.Chat(0, "-debug");
            return debug;
        }

        /// <summary>
        /// A FireAura of range 300 and tick 0.5 s on a, met by b at 100 and c at 200 at its first check (step 16), where c's
        /// pair is paused; b's pair starts a cooldown of a second at each tick it can; then half a second on, at step 32: b
        /// has ticked twice and has half a second of cooldown left, c once.
        /// </summary>
        private static FireAura TwoPairs(FakeDebugWorld debug, Unit a, Unit b, Unit c)
        {
            var aura = new FireAura(300.0f, 0.5f);
            aura.Ticking = pair =>
            {
                if (pair.Other == b)
                {
                    pair.TryCooldown(1.0f);
                }
            };
            a.Interact(aura);
            debug.Advance(0.5f + Step / 2.0f);
            Expect.Equal(2, aura.Pairs.Count, "the aura's pairs at its first check");
            for (var i = 0; i < aura.Pairs.Count; i++)
            {
                if (aura.Pairs[i].Other == c)
                {
                    aura.Pairs[i].Pause();
                }
            }

            debug.Advance(0.5f);
            return aura;
        }

        /// <summary>With only gizmos tracked, a click nearer an item than a gizmo selects the gizmo, and items are still not asked for; once items are, a click nearest the item selects it.</summary>
        public static void CandidatesNoTracking(FakeWorld world)
        {
            var debug = On(world);
            var item = ItemAt(world, 100.0f, 0.0f);
            var well = Gizmo.Create(new Well(), 130.0f, 0.0f);
            debug.CtrlClick(0, 100.0f, 0.0f);
            Expect.Same(well, DebugMode.Selected, "the gizmo is selected, not the item");
            Expect.True(!Tracking.Asked(Tracking.ItemKind), "items are not asked for after the click");
            Expect.True(!Tracking.Asked(Tracking.UnitKind), "units are not asked for after the click");
            Tracking.Need(Tracking.ItemKind);
            debug.CtrlClick(0, 100.0f, 40.0f);
            Expect.Same(item, DebugMode.Selected, "the item, once items are tracked");
        }

        /// <summary>A FireAura holder with two pairs, one in a cooldown and one paused: its cell, the interaction's line, then the pairs nearest first.</summary>
        public static void Lines(FakeWorld world)
        {
            RegisterFootman();
            var debug = On(world);
            var a = FootmanAt(world, 0.0f, 0.0f);
            var b = FootmanAt(world, 100.0f, 0.0f);
            var c = FootmanAt(world, -200.0f, 0.0f);
            TwoPairs(debug, a, b, c);
            DebugMode.Select(a);
            var lines = debug.PanelLines();
            Expect.Equal(6, lines.Count, "the panel's lines");
            Expect.Equal("Footman 1 (unit) at 0, 0 · Player 1", lines[1], "the object line");
            Expect.Equal("cell 8, 8", lines[2], "the cell line");
            Expect.Equal("FireAura: units, range 300, tick 0.5 s, 2 pairs", lines[3], "the interaction's line");
            Expect.Equal("  Footman 2   100 · met 0.5 s ago · ticked 2 · cooldown 0.5 s", lines[4], "the nearer pair, in a cooldown");
            Expect.Equal("  Footman 3   200 · met 0.5 s ago · ticked 1 · paused", lines[5], "the farther pair, paused");
        }

        /// <summary>The other of a pair shows its cell and a "paired by" line naming the interaction and its holder.</summary>
        public static void PairedBy(FakeWorld world)
        {
            RegisterFootman();
            var debug = On(world);
            var a = FootmanAt(world, 0.0f, 0.0f);
            var b = FootmanAt(world, 100.0f, 0.0f);
            var c = FootmanAt(world, -200.0f, 0.0f);
            TwoPairs(debug, a, b, c);
            DebugMode.Select(b);
            var lines = debug.PanelLines();
            Expect.Equal(4, lines.Count, "the panel's lines");
            Expect.Equal("Footman 1 (unit) at 100, 0 · Player 1", lines[1], "the object line");
            Expect.Equal("cell 8, 8", lines[2], "the cell line");
            Expect.Equal("paired by: FireAura on Footman 2 · 100 · met 0.5 s ago", lines[3], "the paired-by line");
        }

        /// <summary>
        /// A FireAura holder at 0, 0 with two pairs: a circle of 300 in 24 lines; the grid lines of the cells within 364
        /// (columns and rows 5 to 10 of 128, so 7 lines each way); a line to the nearer other in Waiting (its cooldown) and to
        /// the farther in Paused. A step later the lines are moved, not drawn again.
        /// </summary>
        public static void Draw(FakeWorld world)
        {
            RegisterFootman();
            var debug = On(world);
            var a = FootmanAt(world, 0.0f, 0.0f);
            var b = FootmanAt(world, 100.0f, 0.0f);
            var c = FootmanAt(world, -200.0f, 0.0f);
            TwoPairs(debug, a, b, c);
            DebugMode.Select(a);
            Expect.Equal(40, debug.Lines.Count, "lines: a circle, the grid lines and two pair lines");
            Expect.Equal(1, debug.Markers.Count, "the marker, core's");
            for (var i = 0; i < 38; i++)
            {
                Expect.Equal(DebugDraw.Neutral, debug.LineStyles[i], "the style of the circle and grid line " + i);
            }

            Expect.Near(300.0f, debug.LineX1s[0], "the circle starts at the range");
            Expect.Near(0.0f, debug.LineY1s[0], "the circle starts on the holder's row");
            Expect.Near(-384.0f, debug.LineX1s[24], "the first grid line's x");
            Expect.Near(-384.0f, debug.LineY1s[24], "the first grid line's bottom");
            Expect.Near(-384.0f, debug.LineX2s[24], "the first grid line is upright");
            Expect.Near(384.0f, debug.LineY2s[24], "the first grid line's top");
            Expect.Near(384.0f, debug.LineX1s[30], "the last upright grid line's x");
            Expect.Near(-384.0f, debug.LineY1s[31], "the first level grid line's y");
            Expect.Near(-384.0f, debug.LineX1s[31], "the first level grid line's left");
            Expect.Near(384.0f, debug.LineX2s[31], "the first level grid line's right");
            Expect.Near(384.0f, debug.LineY1s[37], "the last level grid line's y");
            Expect.Equal(DebugDraw.Waiting, debug.LineStyles[38], "the nearer pair's line, in a cooldown");
            Expect.Near(100.0f, debug.LineX2s[38], "the nearer pair's line ends at its other");
            Expect.Equal(DebugDraw.Paused, debug.LineStyles[39], "the farther pair's line, paused");
            Expect.Near(-200.0f, debug.LineX2s[39], "the farther pair's line ends at its other");
            var added = debug.LinesAdded;
            debug.Advance(Step);
            Expect.Equal(added, debug.LinesAdded, "lines drawn a step later");
            Expect.Equal(40, debug.Lines.Count, "lines a step later");
        }

        /// <summary>A holder with 40 pairs draws 32 pair lines, to the 32 nearest others, nearest first.</summary>
        public static void PairLimit(FakeWorld world)
        {
            RegisterFootman();
            var debug = On(world);
            var a = FootmanAt(world, 0.0f, 0.0f);
            for (var i = 0; i < 40; i++)
            {
                // 7 and 40 share no factor, so the others are placed out of the order of their distances.
                FootmanAt(world, 50.0f + 5.0f * ((i * 7) % 40), 0.0f);
            }

            var aura = new FireAura(300.0f, 0.5f);
            a.Interact(aura);
            debug.Advance(0.5f + Step / 2.0f);
            Expect.Equal(40, aura.Pairs.Count, "the aura's pairs");
            DebugMode.Select(a);
            Expect.Equal(24 + 14 + 32, debug.Lines.Count, "lines: a circle, the grid lines and 32 pair lines");
            for (var k = 0; k < 32; k++)
            {
                Expect.Equal(DebugDraw.Ticking, debug.LineStyles[38 + k], "the style of pair line " + k);
                Expect.Near(50.0f + 5.0f * k, debug.LineX2s[38 + k], "the other of pair line " + k);
            }
        }

        /// <summary>A selected well holding a Healing that ends in its own OnTick: the panel says "Well 1 ended", its drawings and the marker go, the selection is let go, and no error is reported.</summary>
        public static void GizmoEnds(FakeWorld world)
        {
            RegisterFootman();
            var debug = On(world);
            FootmanAt(world, 100.0f, 0.0f);
            var well = Gizmo.Create(new Well(Step), 0.0f, 0.0f);
            well.Interact(new Healing());
            var ending = false;
            well.Ticking = () =>
            {
                if (ending)
                {
                    well.Remove();
                }
            };
            debug.Advance(4.0f * Step);
            debug.CtrlClick(0, 10.0f, 0.0f);
            Expect.Same(well, DebugMode.Selected, "the well is selected");
            var lines = debug.PanelLines();
            Expect.True(lines[1].StartsWith("Well 1 (gizmo) at 0, 0"), "the object line: " + lines[1]);
            Expect.Equal("cell 8, 8", lines[2], "the cell line");
            Expect.Equal("Healing: units, range 300, tick 0.03 s, 1 pair", lines[3], "the interaction's line");
            Expect.True(lines[4].StartsWith("  Footman 1   100 · met "), "the pair's line: " + lines[4]);
            Expect.Equal(1, debug.Markers.Count, "the marker on the well");
            Expect.Near(0.0f, debug.MarkerXs[0], "the marker's x, the place the view gives");
            Expect.Equal(24 + 14 + 1, debug.Lines.Count, "lines while selected");
            var logged = world.Logged.Count;
            ending = true;
            debug.Advance(2.0f * Step);
            lines = debug.PanelLines();
            Expect.True(well.HasEnded, "the well has ended");
            Expect.Equal(2, lines.Count, "the panel's lines");
            Expect.Equal("Well 1 ended", lines[1], "the line of the ended well");
            Expect.Equal(0, debug.Lines.Count, "lines after the well ended");
            Expect.Equal(0, debug.Markers.Count, "markers after the well ended");
            Expect.Same(null, DebugMode.Selected, "the selection after the well ended");
            Expect.Equal(logged, world.Logged.Count, "errors logged");
        }

        /// <summary>
        /// 50 footmen in 25 twos, far apart, each holding a FireAura that pairs with the other of its two; a well holding a
        /// Healing with two footmen in range, started first; a well that moves os.clock 2 ms at every step; 48 more wells.
        /// After three seconds, -stats gives the cycle's cost, the counts tracked, the interactions, pairs and ticks, and a
        /// line per class, most pairs first.
        /// </summary>
        public static void Stats(FakeWorld world)
        {
            RegisterFootman();
            var debug = On(world);
            var healer = Gizmo.Create(new Well(), -989.0f, -1014.0f);
            healer.Interact(new Healing());
            for (var column = 0; column < 15; column += 3)
            {
                for (var row = 0; row < 15; row += 3)
                {
                    var x = -1024.0f + 128.0f * column + 10.0f;
                    var y = -1024.0f + 128.0f * row + 10.0f;
                    FootmanAt(world, x, y).Interact(new FireAura(300.0f, 0.5f));
                    FootmanAt(world, x + 50.0f, y).Interact(new FireAura(300.0f, 0.5f));
                }
            }

            var clock = Gizmo.Create(new Well(Step), 778.0f, 778.0f);
            clock.Ticking = () => { debug.ClockSeconds += 0.002f; };
            for (var i = 0; i < 48; i++)
            {
                Gizmo.Create(new Well(), 779.0f + i, 778.0f);
            }

            debug.Advance(3.0f + Step / 2.0f);
            debug.Chat(0, "-stats");
            var lines = debug.PanelLines();
            Expect.Equal(7, lines.Count, "the panel's lines");
            Expect.Equal("cycle 2.0 ms a step (worst 2) · 32 steps a second", lines[1], "the cycle's line");
            Expect.Equal("tracked: 50 units, 0 items, 0 destructables, 50 gizmos · 26 cells", lines[2], "the tracked line");
            Expect.Equal("interactions 51 · pairs 52 · ticks 164 a second", lines[3], "the counts' line");
            Expect.Equal("FireAura  50 · 50 pairs · 100 ticks a second", lines[4], "the FireAura line, most pairs");
            Expect.Equal("Healing  1 · 2 pairs · 64 ticks a second", lines[5], "the Healing line");
            Expect.True(lines[6].StartsWith("Moonwell's timers "), "core's timers line: " + lines[6]);
        }

        /// <summary>The view is added once however often the package is installed, and again after a reset; Interactions.Step is the debug mode's step: Ctrl + N halted with units running moves Moonwell's time 0.05 s.</summary>
        public static void StepLength(FakeWorld world)
        {
            Interactions.Install(new FakeInteractionWorld());
            Expect.Equal(1, DebugViews.All.Count, "views after an install");
            Runtime.Reset(world);
            Expect.Equal(0, DebugViews.All.Count, "views after a reset");
            Interactions.Step = 0.05f;
            Interactions.Install(new FakeInteractionWorld());
            Interactions.Install(new FakeInteractionWorld());
            Expect.Equal(1, DebugViews.All.Count, "views after a reset and two installs");
            Expect.Near(0.05f, DebugViews.StepLength, "the step length");
            var debug = FakeDebugWorld.Install(world);
            debug.Chat(0, "-debug");
            debug.Advance(1.0f);
            debug.Ctrl(0, DebugKey.H, shift: true);
            var before = Runtime.World.Now();
            debug.Ctrl(0, DebugKey.N);
            Expect.Near(before + 0.05f, Runtime.World.Now(), "Moonwell's time after Ctrl + N");
        }
    }
#endif
}
