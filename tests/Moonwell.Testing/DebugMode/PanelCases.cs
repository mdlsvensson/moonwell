using Moonwell.Hosting;

namespace Moonwell.Testing
{
#if DEBUG
    /// <summary>Cases for the debug mode's panel, drawing and statistics: the status line, the selected object's lines, numbers, the cap, the refresh, an ended object, turning off, reused lines and -stats.</summary>
    public static class PanelCases
    {
        /// <summary>One step of Moonwell's time when no view sets it.</summary>
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

        /// <summary>Installs the fake debug world and turns the mode on with -debug.</summary>
        private static FakeDebugWorld On(FakeWorld world)
        {
            var debug = FakeDebugWorld.Install(world);
            debug.Chat(0, "-debug");
            return debug;
        }

        /// <summary>
        /// Halted at step 3 with units held, the first line reads "DEBUG  halted · step 3 (0.1 s) · speed 1 · units held";
        /// resumed, "running"; halted with units running at 37.625 s, step 1,204 (37.6 s) and "units running"; the speed
        /// follows Ctrl + = and -speed.
        /// </summary>
        public static void Status(FakeWorld world)
        {
            var debug = On(world);
            Expect.True(debug.PanelShown, "the panel is shown after -debug");
            debug.Ctrl(0, DebugKey.H);
            for (var i = 0; i < 3; i++)
            {
                debug.Ctrl(0, DebugKey.N);
                debug.Advance(Step);
            }

            Expect.Equal("DEBUG  halted · step 3 (0.1 s) · speed 1 · units held", debug.PanelLines()[0], "the status halted at step 3 with units held");
            debug.Ctrl(0, DebugKey.H);
            Expect.Equal("DEBUG  running · step 3 (0.1 s) · speed 1", debug.PanelLines()[0], "the status after the resume");
            debug.Advance(37.53125f);
            debug.Ctrl(0, DebugKey.H, shift: true);
            Expect.Equal("DEBUG  halted · step 1,204 (37.6 s) · speed 1 · units running", debug.PanelLines()[0], "the status halted at 37.625 s with units running");
            debug.Ctrl(0, DebugKey.H, shift: true);
            debug.Ctrl(0, DebugKey.Plus);
            Expect.Equal("DEBUG  running · step 1,204 (37.6 s) · speed 2", debug.PanelLines()[0], "the status after Ctrl + =");
            debug.Chat(0, "-speed 1/4");
            Expect.Equal("DEBUG  running · step 1,204 (37.6 s) · speed 1/4", debug.PanelLines()[0], "the status after -speed 1/4");
        }

        /// <summary>A selected footman with a line of its own and a component that adds "kills 3": the object line (no scope: the game made it), the footman's line, then "kills 3"; the marker on the footman.</summary>
        public static void SelectedUnit(FakeWorld world)
        {
            RegisterFootman();
            var footman = FootmanAt(world, 100.0f, -200.0f);
            footman.Line = "trained 2";
            var shown = footman.Add(new Shown());
            shown.Lines.Add("kills 3");
            var debug = On(world);
            debug.CtrlClick(0, 100.0f, -200.0f);
            var lines = debug.PanelLines();
            Expect.Equal(4, lines.Count, "the panel's lines");
            Expect.Equal("Footman 1 (unit) at 100, -200 · Player 1", lines[1], "the object line of a unit the game made, which has no owner");
            Expect.Equal("trained 2", lines[2], "the footman's own line");
            Expect.Equal("kills 3", lines[3], "the component's line");
            Expect.Equal(1, debug.Markers.Count, "markers");
            Expect.Near(100.0f, debug.MarkerXs[0], "the marker's x");
            Expect.Near(-200.0f, debug.MarkerYs[0], "the marker's y");
        }

        /// <summary>Two footmen named get 1 and 2, and the first keeps 1 when named again; a footman owned by the first names it "Footman 1".</summary>
        public static void Numbers(FakeWorld world)
        {
            RegisterFootman();
            var first = FootmanAt(world, 0.0f, 0.0f);
            var second = FootmanAt(world, 500.0f, 0.0f);
            var debug = On(world);
            DebugMode.Select(first);
            Expect.True(debug.PanelLines()[1].StartsWith("Footman 1 (unit)"), "the first footman's line: " + debug.PanelLines()[1]);
            DebugMode.Select(second);
            Expect.True(debug.PanelLines()[1].StartsWith("Footman 2 (unit)"), "the second footman's line: " + debug.PanelLines()[1]);
            DebugMode.Select(first);
            Expect.True(debug.PanelLines()[1].StartsWith("Footman 1 (unit)"), "the first footman's line again: " + debug.PanelLines()[1]);
            var third = Unit.Create(ObjectTypes.FourCC("hfoo"), Player.Two, 0.0f, 0.0f, first);
            world.PlaceUnit(third.Handle, 1000.4f, 1234.6f);
            DebugMode.Select(third);
            Expect.Equal("Footman 3 (unit) at 1,000, 1,235 · Player 2 · in Footman 1", debug.PanelLines()[1], "the third footman's line");
        }

        /// <summary>40 lines from IDebugShown, with the status and object lines 42: 30 lines shown, the last "and 13 more".</summary>
        public static void Cap(FakeWorld world)
        {
            RegisterFootman();
            var footman = FootmanAt(world, 0.0f, 0.0f);
            var shown = footman.Add(new Shown());
            for (var i = 1; i <= 40; i++)
            {
                shown.Lines.Add("line " + i);
            }

            var debug = On(world);
            DebugMode.Select(footman);
            var lines = debug.PanelLines();
            Expect.Equal(30, lines.Count, "the panel's lines");
            Expect.Equal("line 27", lines[28], "the last line shown");
            Expect.Equal("and 13 more", lines[29], "the last line");
        }

        /// <summary>A ShowDebug that throws is reported as Shown.ShowDebug; the lines it added, and the next component's, are still in the panel.</summary>
        public static void ThrowingShown(FakeWorld world)
        {
            RegisterFootman();
            var footman = FootmanAt(world, 0.0f, 0.0f);
            var thrower = footman.Add(new Shown { Throws = true });
            thrower.Lines.Add("before");
            var next = footman.Add(new Shown());
            next.Lines.Add("kills 3");
            var debug = On(world);
            var logged = world.Logged.Count;
            DebugMode.Select(footman);
            Expect.Equal(logged + 1, world.Logged.Count, "errors logged");
            Expect.True(world.Logged[logged].StartsWith("Moonwell: Shown.ShowDebug failed"), "the error logged: " + world.Logged[logged]);
            var lines = debug.PanelLines();
            Expect.Equal(4, lines.Count, "the panel's lines");
            Expect.True(lines[1].StartsWith("Footman 1 (unit)"), "the object line");
            Expect.Equal("before", lines[2], "the line added before the throw");
            Expect.Equal("kills 3", lines[3], "the next component's line");
        }

        /// <summary>Running, the text is set 4 times a second and the drawings move every 1/32 s; halted, the text is set at the halt and at each step's end, and nothing moves between.</summary>
        public static void Refresh(FakeWorld world)
        {
            DebugViews.Add(new CircleView());
            RegisterFootman();
            var footman = FootmanAt(world, 0.0f, 0.0f);
            var debug = On(world);
            debug.CtrlClick(0, 0.0f, 0.0f);
            Expect.Equal(24, debug.Lines.Count, "lines of the circle");
            var sets = debug.PanelTextSets;
            var moved = debug.LinesMoved;
            debug.Advance(1.0f);
            Expect.Equal(sets + 4, debug.PanelTextSets, "texts set in a second running");
            Expect.Equal(moved + 24 * 32, debug.LinesMoved, "line moves in a second running");
            debug.Ctrl(0, DebugKey.H);
            Expect.Equal(sets + 5, debug.PanelTextSets, "texts set after the halt");
            moved = debug.LinesMoved;
            debug.Advance(1.0f);
            Expect.Equal(sets + 5, debug.PanelTextSets, "texts set in a second halted");
            Expect.Equal(moved, debug.LinesMoved, "line moves in a second halted");
            debug.Ctrl(0, DebugKey.N);
            Expect.Equal(sets + 5, debug.PanelTextSets, "texts set as a held step starts");
            debug.Advance(Step);
            Expect.Equal(sets + 6, debug.PanelTextSets, "texts set at the step's end");
            Expect.Equal(moved + 24, debug.LinesMoved, "line moves at the step's end");
            debug.Advance(1.0f);
            Expect.Equal(sets + 6, debug.PanelTextSets, "texts set in a second halted after the step");
            Expect.Same(footman, DebugMode.Selected, "the selection");
        }

        /// <summary>The selected unit removed while halted: the panel says "Footman 1 ended", its marker and circle go, the selection is cleared, and no error is reported.</summary>
        public static void Ended(FakeWorld world)
        {
            DebugViews.Add(new CircleView());
            RegisterFootman();
            var footman = FootmanAt(world, 0.0f, 0.0f);
            var debug = On(world);
            debug.CtrlClick(0, 0.0f, 0.0f);
            Expect.Equal(1, debug.Markers.Count, "markers while selected");
            debug.Ctrl(0, DebugKey.H);
            var logged = world.Logged.Count;
            footman.Remove();
            debug.Advance(Step);
            var lines = debug.PanelLines();
            Expect.Equal(2, lines.Count, "the panel's lines");
            Expect.Equal("Footman 1 ended", lines[1], "the line of the ended unit");
            Expect.Equal(0, debug.Markers.Count, "markers after the unit ended");
            Expect.Equal(0, debug.Lines.Count, "lines after the unit ended");
            Expect.Same(null, DebugMode.Selected, "the selection after the unit ended");
            Expect.Equal(logged, world.Logged.Count, "errors logged");
            debug.Ctrl(0, DebugKey.N);
            debug.Advance(Step);
            Expect.Equal("Footman 1 ended", debug.PanelLines()[1], "the line after a step");
        }

        /// <summary>Halted with units running, a selected footman the game moves: its marker follows each 1/32 s and the text is set 4 times a second; halted with units held, the marker stays.</summary>
        public static void FreeHaltFollows(FakeWorld world)
        {
            RegisterFootman();
            var footman = FootmanAt(world, 0.0f, 0.0f);
            var debug = On(world);
            debug.CtrlClick(0, 0.0f, 0.0f);
            debug.Ctrl(0, DebugKey.H, shift: true);
            var sets = debug.PanelTextSets;
            world.PlaceUnit(footman.Handle, 200.0f, 0.0f);
            debug.Advance(Step);
            Expect.Near(200.0f, debug.MarkerXs[0], "the marker's x a step after the footman moved, halted with units running");
            debug.Advance(1.0f - Step);
            Expect.Equal(sets + 4, debug.PanelTextSets, "texts set in a second halted with units running");
            Expect.Equal("Footman 1 (unit) at 200, 0 · Player 1", debug.PanelLines()[1], "the object line, halted with units running");
            debug.Ctrl(0, DebugKey.H, shift: true);
            debug.Ctrl(0, DebugKey.H);
            sets = debug.PanelTextSets;
            world.PlaceUnit(footman.Handle, 400.0f, 0.0f);
            debug.Advance(1.0f);
            Expect.Near(200.0f, debug.MarkerXs[0], "the marker's x a second after the footman moved, halted with units held");
            Expect.Equal(sets, debug.PanelTextSets, "texts set in a second halted with units held");
        }

        /// <summary>-debug off hides the panel and removes every line and marker, and nothing is drawn or set after; on again shows it.</summary>
        public static void Off(FakeWorld world)
        {
            DebugViews.Add(new CircleView());
            RegisterFootman();
            FootmanAt(world, 0.0f, 0.0f);
            var debug = On(world);
            debug.CtrlClick(0, 0.0f, 0.0f);
            Expect.Equal(24, debug.Lines.Count, "lines while on");
            Expect.Equal(1, debug.Markers.Count, "markers while on");
            debug.Ctrl(0, DebugKey.H);
            debug.Chat(0, "-debug");
            Expect.True(!debug.PanelShown, "the panel is hidden after -debug off");
            Expect.Equal(0, debug.Lines.Count, "lines after -debug off");
            Expect.Equal(0, debug.Markers.Count, "markers after -debug off");
            var sets = debug.PanelTextSets;
            var added = debug.LinesAdded;
            debug.Advance(1.0f);
            Expect.Equal(sets, debug.PanelTextSets, "texts set in a second off");
            Expect.Equal(added, debug.LinesAdded, "lines drawn in a second off");
            debug.Chat(0, "-debug");
            Expect.True(debug.PanelShown, "the panel after -debug on again");
            Expect.Equal(3, debug.PanelShows, "times the panel was shown or hidden");
            Expect.Equal(1, debug.PanelLines().Count, "the panel's lines with nothing selected");
        }

        /// <summary>Two frames of the same three lines add 3 and move 3; a frame of 1 moves it and removes 2; a line whose style changes is drawn again; markers are reused the same way; a circle is 24 lines.</summary>
        public static void Reuse(FakeWorld world)
        {
            var debug = new FakeDebugWorld(world);
            var draw = new DebugDraw(debug);
            draw.Begin();
            draw.Line(DebugDraw.Neutral, 0.0f, 0.0f, 100.0f, 0.0f);
            draw.Line(DebugDraw.Ticking, 0.0f, 0.0f, 0.0f, 100.0f);
            draw.Line(DebugDraw.Waiting, 0.0f, 0.0f, 100.0f, 100.0f);
            draw.End();
            Expect.Equal(3, debug.LinesAdded, "lines drawn by the first frame");
            Expect.Equal(0, debug.LinesMoved, "lines moved by the first frame");
            draw.Begin();
            draw.Line(DebugDraw.Neutral, 10.0f, 0.0f, 110.0f, 0.0f);
            draw.Line(DebugDraw.Ticking, 10.0f, 0.0f, 10.0f, 100.0f);
            draw.Line(DebugDraw.Waiting, 10.0f, 0.0f, 110.0f, 100.0f);
            draw.End();
            Expect.Equal(3, debug.LinesAdded, "lines drawn after the second frame");
            Expect.Equal(3, debug.LinesMoved, "lines moved by the second frame");
            Expect.Equal(0, debug.LinesRemoved, "lines removed by the second frame");
            draw.Begin();
            draw.Line(DebugDraw.Neutral, 20.0f, 0.0f, 120.0f, 0.0f);
            draw.End();
            Expect.Equal(4, debug.LinesMoved, "lines moved after the third frame");
            Expect.Equal(2, debug.LinesRemoved, "lines removed by the third frame");
            Expect.Equal(1, debug.Lines.Count, "lines after the third frame");
            draw.Begin();
            draw.Line(DebugDraw.Paused, 20.0f, 0.0f, 120.0f, 0.0f);
            draw.End();
            Expect.Equal(4, debug.LinesAdded, "lines drawn after a frame of another style");
            Expect.Equal(3, debug.LinesRemoved, "lines removed after a frame of another style");
            Expect.Equal(DebugDraw.Paused, debug.LineStyles[0], "the style drawn");
            draw.Begin();
            draw.Marker(10.0f, 20.0f);
            draw.End();
            Expect.Equal(0, debug.Lines.Count, "lines after a frame of a marker only");
            Expect.Equal(1, debug.MarkersAdded, "markers put by the first frame with one");
            draw.Begin();
            draw.Marker(30.0f, 40.0f);
            draw.End();
            Expect.Equal(1, debug.MarkersAdded, "markers put after a second frame with one");
            Expect.Near(30.0f, debug.MarkerXs[0], "the marker's x after the second frame");
            Expect.Near(40.0f, debug.MarkerYs[0], "the marker's y after the second frame");
            draw.Begin();
            draw.Circle(DebugDraw.Neutral, 0.0f, 0.0f, 100.0f);
            draw.End();
            Expect.Equal(1, debug.MarkersRemoved, "markers removed by a frame without one");
            Expect.Equal(24, debug.Lines.Count, "lines of a circle");
        }

        /// <summary>-stats adds the views' statistics and the timers' line (4 wakes of 2 ms a second: 8.0 ms) and logs them once; again hides them; a third time logs them again.</summary>
        public static void Stats(FakeWorld world)
        {
            DebugViews.Add(new CircleView());
            var debug = On(world);
            debug.Advance(0.01f);
            Game.Instance.Every(0.25f, () => { debug.ClockSeconds += 0.002f; });
            debug.Advance(2.0f);
            var printed = world.PrintedTexts.Count;
            debug.Chat(0, "-stats");
            Expect.Equal(printed + 1, world.PrintedTexts.Count, "lines printed by -stats");
            var lines = debug.PanelLines();
            Expect.Equal(3, lines.Count, "the panel's lines with -stats");
            Expect.Equal("circles 1", lines[1], "the view's statistics");
            Expect.Equal("Moonwell's timers 8.0 ms a second", lines[2], "the timers' line");
            Expect.Equal(2, debug.Logged.Count, "lines logged by -stats");
            Expect.Equal("Moonwell's timers 8.0 ms a second", debug.Logged[1], "the timers' line logged");
            debug.Advance(1.0f);
            Expect.Equal(2, debug.Logged.Count, "lines logged a second later");
            debug.Chat(0, "-stats");
            Expect.Equal(1, debug.PanelLines().Count, "the panel's lines after -stats again");
            Expect.Equal(2, debug.Logged.Count, "lines logged after -stats again");
            debug.Chat(0, "-stats");
            Expect.Equal(4, debug.Logged.Count, "lines logged after a third -stats");
        }
    }
#endif
}
