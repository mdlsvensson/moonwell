using Moonwell.Hosting;

namespace Moonwell.Testing
{
#if DEBUG
    /// <summary>Cases for the debug mode's session: -debug, its keys and chat commands, Ctrl+click selection, and halting, stepping and changing speed with units held or running.</summary>
    public static class SessionCases
    {
        /// <summary>One step of Moonwell's time when no view sets it.</summary>
        private const float Step = 1.0f / 32.0f;

        /// <summary>A unit of the footman type taken in at a place.</summary>
        private static Unit UnitAt(FakeWorld world, float x, float y)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            world.PlaceUnit(handle, x, y);
            return Unit.Of(handle)!;
        }

        /// <summary>Installs the fake debug world and turns the mode on with -debug.</summary>
        private static FakeDebugWorld On(FakeWorld world)
        {
            var debug = FakeDebugWorld.Install(world);
            debug.Chat(0, "-debug");
            return debug;
        }

        /// <summary>Whether the fake world has a unit paused.</summary>
        private static bool Paused(FakeWorld world, Unit unit)
        {
            return world.IsPausedUnit(unit.Handle);
        }

        /// <summary>At the start only the chat is listened to, and Ctrl + H does nothing.</summary>
        public static void OffAtStart(FakeWorld world)
        {
            var debug = FakeDebugWorld.Install(world);
            var unit = UnitAt(world, 0.0f, 0.0f);
            Expect.Equal(1, debug.ChatListens, "chat listens at the start");
            Expect.Equal(0, debug.InputListens, "input listens at the start");
            Expect.True(!DebugSession.IsOn, "the mode is off at the start");
            debug.Ctrl(0, DebugKey.H);
            Expect.True(!Runtime.Clock.IsHalted, "the clock runs after Ctrl + H while off");
            Expect.True(!Paused(world, unit), "the unit runs after Ctrl + H while off");
            debug.Chat(0, "-speed 4");
            Expect.Near(1.0f, Runtime.Clock.Speed, "the speed after -speed 4 while off");
        }

        /// <summary>
        /// Only lines that start with "-" reach the session, and it trims them: "hello -debug" and " -debug" do nothing,
        /// "-debug " turns the mode on and listens to the keys and the mouse once; again turns it off; a third time on, with
        /// no second listen.
        /// </summary>
        public static void Toggle(FakeWorld world)
        {
            var debug = FakeDebugWorld.Install(world);
            var printed = world.PrintedTexts.Count;
            debug.Chat(0, "hello -debug");
            Expect.True(!DebugSession.IsOn, "the mode after \"hello -debug\"");
            debug.Chat(0, " -debug");
            Expect.True(!DebugSession.IsOn, "the mode after \" -debug\"");
            Expect.Equal(printed, world.PrintedTexts.Count, "lines printed by lines that do not start with -");
            debug.Chat(3, "-debug ");
            Expect.True(DebugSession.IsOn, "the mode after \"-debug \"");
            Expect.Equal(1, debug.InputListens, "input listens after -debug");
            Expect.Equal(printed + 1, world.PrintedTexts.Count, "lines printed by -debug");
            Expect.Equal(-1, world.PrintedSlots[world.PrintedSlots.Count - 1], "the line is printed to everyone");
            debug.Chat(0, "-debug");
            Expect.True(!DebugSession.IsOn, "the mode after -debug again");
            Expect.Equal(printed + 2, world.PrintedTexts.Count, "lines printed by -debug again");
            debug.Chat(0, "-debug");
            Expect.True(DebugSession.IsOn, "the mode after a third -debug");
            Expect.Equal(1, debug.InputListens, "input listens after a third -debug");
            Expect.Equal(1, debug.ChatListens, "chat listens after a third -debug");
        }

        /// <summary>Ctrl + H halts the clock and holds every unit; again, resumes and lets them go.</summary>
        public static void HaltHolds(FakeWorld world)
        {
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 100.0f, 0.0f);
            var debug = On(world);
            var printed = world.PrintedTexts.Count;
            debug.Ctrl(0, DebugKey.H);
            Expect.True(Runtime.Clock.IsHalted, "the clock after Ctrl + H");
            Expect.True(Paused(world, a) && Paused(world, b), "both units are held after Ctrl + H");
            Expect.True(!world.WakeAt.HasValue, "the game wake armed before Ctrl + H is cancelled");
            Expect.Equal(printed + 1, world.PrintedTexts.Count, "lines printed by Ctrl + H");
            debug.Ctrl(0, DebugKey.H);
            Expect.True(!Runtime.Clock.IsHalted, "the clock after Ctrl + H again");
            Expect.True(!Paused(world, a) && !Paused(world, b), "both units are let go after Ctrl + H again");
            Expect.Equal(printed + 2, world.PrintedTexts.Count, "lines printed by Ctrl + H again");
        }

        /// <summary>Ctrl + Shift + H halts the clock and leaves the units running; again, resumes.</summary>
        public static void HaltFree(FakeWorld world)
        {
            var a = UnitAt(world, 0.0f, 0.0f);
            var debug = On(world);
            debug.Ctrl(0, DebugKey.H, shift: true);
            Expect.True(Runtime.Clock.IsHalted, "the clock after Ctrl + Shift + H");
            Expect.True(!Paused(world, a), "the unit runs after Ctrl + Shift + H");
            Expect.Equal(0, debug.PauseCalls.Count, "pause calls after Ctrl + Shift + H");
            debug.Ctrl(0, DebugKey.H, shift: true);
            Expect.True(!Runtime.Clock.IsHalted, "the clock after Ctrl + Shift + H again");
        }

        /// <summary>A unit the map paused before Ctrl + H is still paused after the resume; the other is let go.</summary>
        public static void MapPausedStays(FakeWorld world)
        {
            var mapPaused = UnitAt(world, 0.0f, 0.0f);
            var other = UnitAt(world, 100.0f, 0.0f);
            mapPaused.IsPaused = true;
            var debug = On(world);
            debug.Ctrl(0, DebugKey.H);
            Expect.True(Paused(world, mapPaused) && Paused(world, other), "both units are paused while halted");
            debug.Ctrl(0, DebugKey.H);
            Expect.True(Paused(world, mapPaused), "the unit the map paused after the resume");
            Expect.True(!Paused(world, other), "the other unit after the resume");
            Expect.Equal(2, debug.PauseCalls.Count, "pause calls: the other unit's pause and let go only");
            Expect.Same(other.Handle, debug.PausedOrLetGo[0], "the unit paused");
        }

        /// <summary>A unit taken in while units are held is held at once, and let go at the resume.</summary>
        public static void NewUnitHeld(FakeWorld world)
        {
            var debug = On(world);
            debug.Ctrl(0, DebugKey.H);
            var late = UnitAt(world, 0.0f, 0.0f);
            Expect.True(Paused(world, late), "the unit taken in while held");
            debug.Ctrl(0, DebugKey.H);
            Expect.True(!Paused(world, late), "the unit taken in while held, after the resume");
            var free = UnitAt(world, 0.0f, 0.0f);
            Expect.True(!Paused(world, free), "a unit taken in while running");
        }

        /// <summary>Halted with units running, Ctrl + N moves Moonwell's time one step at once and runs what is due in it; running, it does nothing.</summary>
        public static void StepFree(FakeWorld world)
        {
            var debug = On(world);
            world.Advance(1.0f);
            debug.Ctrl(0, DebugKey.N);
            Expect.Near(1.0f, Runtime.World.Now(), "Moonwell's time after Ctrl + N while running");
            debug.Ctrl(0, DebugKey.H, shift: true);
            var ran = 0;
            Timer.After(Step, () => { ran++; });
            var before = Runtime.World.Now();
            var printed = world.PrintedTexts.Count;
            debug.Ctrl(0, DebugKey.N);
            Expect.Near(before + Step, Runtime.World.Now(), "Moonwell's time after Ctrl + N");
            Expect.Equal(1, ran, "runs of a timer due one step on");
            Expect.True(Runtime.Clock.IsHalted, "the clock after the step");
            Expect.True(!DebugSession.StepRunning, "a step runs after a free step");
            Expect.Equal(printed, world.PrintedTexts.Count, "lines printed by a step");
        }

        /// <summary>Halted with units held, Ctrl + N lets the units go for one step of game time, then holds them again with Moonwell's time one step on.</summary>
        public static void StepHeld(FakeWorld world)
        {
            var unit = UnitAt(world, 0.0f, 0.0f);
            var debug = On(world);
            world.Advance(1.0f);
            debug.Ctrl(0, DebugKey.H);
            debug.Advance(0.5f);
            var before = Runtime.World.Now();
            debug.Ctrl(0, DebugKey.N);
            Expect.True(!Paused(world, unit), "the unit during the step");
            Expect.True(DebugSession.StepRunning, "a step runs");
            Expect.True(Runtime.Clock.IsHalted, "the clock during the step");
            debug.Advance(Step / 2.0f);
            Expect.True(!Paused(world, unit), "the unit half a step in");
            Expect.Near(before, Runtime.World.Now(), "Moonwell's time half a step in");
            debug.Advance(Step / 2.0f);
            Expect.True(Paused(world, unit), "the unit after the step");
            Expect.True(!DebugSession.StepRunning, "a step runs after the step");
            Expect.True(Runtime.Clock.IsHalted, "the clock after the step");
            Expect.Near(before + Step, Runtime.World.Now(), "Moonwell's time after the step");
            debug.Advance(1.0f);
            Expect.Near(before + Step, Runtime.World.Now(), "Moonwell's time a second after the step");
        }

        /// <summary>A second Ctrl + N while a held step runs is dropped; two steps taken one after the other move two steps.</summary>
        public static void StepDropped(FakeWorld world)
        {
            UnitAt(world, 0.0f, 0.0f);
            var debug = On(world);
            debug.Ctrl(0, DebugKey.H);
            var before = Runtime.World.Now();
            debug.Ctrl(0, DebugKey.N);
            debug.Ctrl(0, DebugKey.N);
            debug.Advance(Step);
            Expect.Near(before + Step, Runtime.World.Now(), "Moonwell's time after a step with a second press in it");
            debug.Advance(Step * 4.0f);
            Expect.Near(before + Step, Runtime.World.Now(), "Moonwell's time four steps of game time later");
            debug.Ctrl(0, DebugKey.N);
            debug.Advance(Step);
            debug.Ctrl(0, DebugKey.N);
            debug.Advance(Step);
            Expect.Near(before + Step * 3.0f, Runtime.World.Now(), "Moonwell's time after two more steps");
        }

        /// <summary>Ctrl + H while a held step runs resumes: the units stay let go, and the step's end does nothing.</summary>
        public static void ResumeMidStep(FakeWorld world)
        {
            var unit = UnitAt(world, 0.0f, 0.0f);
            var debug = On(world);
            debug.Ctrl(0, DebugKey.H);
            debug.Ctrl(0, DebugKey.N);
            debug.Ctrl(0, DebugKey.H);
            Expect.True(!Runtime.Clock.IsHalted, "the clock after Ctrl + H mid-step");
            Expect.True(!DebugSession.StepRunning, "a step runs after Ctrl + H mid-step");
            var before = Runtime.World.Now();
            debug.Advance(Step);
            Expect.True(!Paused(world, unit), "the unit after the step's time");
            Expect.True(!Runtime.Clock.IsHalted, "the clock after the step's time");
            Expect.Near(before + Step, Runtime.World.Now(), "Moonwell's time runs with the game's");
        }

        /// <summary>Ctrl + = from 1 goes 2, 4, 8, 8; -speed 1 sets 1; Ctrl + - goes ½, ¼, ¼; -speed 3 prints a line and changes nothing.</summary>
        public static void Speeds(FakeWorld world)
        {
            var debug = On(world);
            var expected = new float[] { 2.0f, 4.0f, 8.0f, 8.0f };
            for (var i = 0; i < expected.Length; i++)
            {
                debug.Ctrl(0, DebugKey.Plus);
                Expect.Near(expected[i], Runtime.Clock.Speed, "the speed after Ctrl + = " + (i + 1) + " times");
            }

            debug.Chat(0, "-speed 1");
            Expect.Near(1.0f, Runtime.Clock.Speed, "the speed after -speed 1");
            expected = new float[] { 0.5f, 0.25f, 0.25f };
            for (var i = 0; i < expected.Length; i++)
            {
                debug.Ctrl(0, DebugKey.Minus);
                Expect.Near(expected[i], Runtime.Clock.Speed, "the speed after Ctrl + - " + (i + 1) + " times");
            }

            debug.Chat(0, "-speed 1/2");
            Expect.Near(0.5f, Runtime.Clock.Speed, "the speed after -speed 1/2");
            debug.Chat(0, "-speed .25");
            Expect.Near(0.25f, Runtime.Clock.Speed, "the speed after -speed .25");
            debug.Chat(0, "-speed 1/0");
            Expect.Near(0.25f, Runtime.Clock.Speed, "the speed after -speed 1/0");
            debug.Chat(0, "-speed 0.5");
            Expect.Near(0.5f, Runtime.Clock.Speed, "the speed after -speed 0.5");
            var printed = world.PrintedTexts.Count;
            debug.Chat(0, "-speed 3");
            Expect.Near(0.5f, Runtime.Clock.Speed, "the speed after -speed 3");
            Expect.Equal(printed + 1, world.PrintedTexts.Count, "lines printed by -speed 3");
            debug.Chat(0, "-speed fast");
            Expect.Near(0.5f, Runtime.Clock.Speed, "the speed after -speed fast");
            debug.Chat(0, "-speed 4");
            var ran = 0;
            Timer.Every(0.5f, () => { ran++; });
            world.Advance(1.0f);
            Expect.Equal(8, ran, "runs of a 0.5 s timer in 1 second at -speed 4");
        }

        /// <summary>-step 10 moves 10 steps, at once with units running and one after another with units held, and prints "10 steps."; -step 1 prints "1 step."; 0 and 321 print a line and do nothing.</summary>
        public static void StepCount(FakeWorld world)
        {
            var unit = UnitAt(world, 0.0f, 0.0f);
            var debug = On(world);
            debug.Ctrl(0, DebugKey.H, shift: true);
            var before = Runtime.World.Now();
            var printed = world.PrintedTexts.Count;
            debug.Chat(0, "-step 10");
            Expect.Near(before + Step * 10.0f, Runtime.World.Now(), "Moonwell's time after -step 10 with units running");
            Expect.Equal(printed + 1, world.PrintedTexts.Count, "lines printed by -step 10");
            Expect.Equal("10 steps.", world.PrintedTexts[printed], "the line of -step 10");
            debug.Chat(0, "-step 1");
            Expect.Near(before + Step * 11.0f, Runtime.World.Now(), "Moonwell's time after -step 1");
            Expect.Equal("1 step.", world.PrintedTexts[printed + 1], "the line of -step 1");
            before = Runtime.World.Now();
            printed = world.PrintedTexts.Count;
            debug.Chat(0, "-step 0");
            debug.Chat(0, "-step 321");
            Expect.Equal(printed + 2, world.PrintedTexts.Count, "lines printed by -step 0 and -step 321");
            Expect.Near(before, Runtime.World.Now(), "Moonwell's time after -step 0 and -step 321");
            debug.Ctrl(0, DebugKey.H, shift: true);
            debug.Ctrl(0, DebugKey.H);
            before = Runtime.World.Now();
            debug.Chat(0, "-step 10");
            Expect.True(DebugSession.StepRunning, "a step runs after -step 10 with units held");
            debug.Advance(Step * 5.0f);
            Expect.Near(before + Step * 5.0f, Runtime.World.Now(), "Moonwell's time 5 steps of game time into -step 10");
            debug.Advance(1.0f);
            Expect.Near(before + Step * 10.0f, Runtime.World.Now(), "Moonwell's time after -step 10 with units held");
            Expect.True(Paused(world, unit), "the unit after -step 10");
            Expect.True(!DebugSession.StepRunning, "a step runs after -step 10 is done");
        }

        /// <summary>-debug off while halted at speed 4 with units held: the clock runs at 1, the units are let go, nothing is selected.</summary>
        public static void OffRestores(FakeWorld world)
        {
            var unit = UnitAt(world, 0.0f, 0.0f);
            var debug = On(world);
            debug.Ctrl(0, DebugKey.Plus);
            debug.Ctrl(0, DebugKey.Plus);
            debug.CtrlClick(0, 0.0f, 0.0f);
            Expect.Same(unit, DebugSession.Selected, "the unit is selected");
            debug.Ctrl(0, DebugKey.H);
            var before = Runtime.World.Now();
            var printed = world.PrintedTexts.Count;
            debug.Chat(0, "-debug");
            Expect.True(!DebugSession.IsOn, "the mode after -debug");
            Expect.Equal(printed + 1, world.PrintedTexts.Count, "lines printed by -debug off");
            Expect.True(!Runtime.Clock.IsHalted, "the clock after -debug off");
            Expect.Near(1.0f, Runtime.Clock.Speed, "the speed after -debug off");
            Expect.True(!Paused(world, unit), "the unit after -debug off");
            Expect.Same(null, DebugSession.Selected, "the selection after -debug off");
            world.Advance(1.0f);
            Expect.Near(before + 1.0f, Runtime.World.Now(), "Moonwell's time a second after -debug off");
        }

        /// <summary>With Ctrl down, a left press at a unit's place selects it; with Ctrl up, a press at another unit selects nothing.</summary>
        public static void Select(FakeWorld world)
        {
            var a = UnitAt(world, 100.0f, 100.0f);
            var b = UnitAt(world, 600.0f, 100.0f);
            var debug = On(world);
            debug.Key(0, DebugKey.Control, 0, true);
            debug.Press(0, 110.0f, 90.0f);
            Expect.Same(a, DebugSession.Selected, "the unit selected by Ctrl + click");
            debug.Key(0, DebugKey.Control, 2, true);
            debug.Key(0, DebugKey.Control, 0, false);
            debug.Press(0, 600.0f, 100.0f);
            Expect.Same(a, DebugSession.Selected, "the selection after a press with Ctrl up");
            debug.Key(1, DebugKey.Control, 0, true);
            debug.Press(0, 600.0f, 100.0f);
            Expect.Same(a, DebugSession.Selected, "the selection after a press with another player's Ctrl down");
            debug.Press(1, 600.0f, 100.0f);
            Expect.Same(b, DebugSession.Selected, "the unit selected by the other player's Ctrl + click");
        }

        /// <summary>Two units 20 apart: a Ctrl + click selects the nearer, again at the same spot the other, and a third time the nearer again.</summary>
        public static void SelectNext(FakeWorld world)
        {
            var near = UnitAt(world, 20.0f, 0.0f);
            var far = UnitAt(world, 0.0f, 0.0f);
            var debug = On(world);
            debug.CtrlClick(0, 25.0f, 0.0f);
            Expect.Same(near, DebugSession.Selected, "the first click");
            debug.CtrlClick(0, 30.0f, 10.0f);
            Expect.Same(far, DebugSession.Selected, "the second click at the same spot");
            debug.CtrlClick(0, 25.0f, 0.0f);
            Expect.Same(near, DebugSession.Selected, "the third click at the same spot");
            debug.CtrlClick(0, 10.0f, 100.0f);
            Expect.Same(near, DebugSession.Selected, "a click 100 away, where both are as near: the first taken in");
        }

        /// <summary>A Ctrl + click 200 from every unit clears the selection.</summary>
        public static void SelectNothing(FakeWorld world)
        {
            var unit = UnitAt(world, 0.0f, 0.0f);
            var debug = On(world);
            debug.CtrlClick(0, 0.0f, 0.0f);
            Expect.Same(unit, DebugSession.Selected, "the unit is selected");
            debug.CtrlClick(0, 200.0f, 0.0f);
            Expect.Same(null, DebugSession.Selected, "the selection after a click 200 away");
        }

        /// <summary>DebugMode.Halt() while off turns the mode on as -debug does, its keys listened to and its line printed, then halts with units held; Ctrl + H then resumes.</summary>
        public static void HaltFromCodeTurnsOn(FakeWorld world)
        {
            var unit = UnitAt(world, 0.0f, 0.0f);
            var debug = FakeDebugWorld.Install(world);
            var printed = world.PrintedTexts.Count;
            DebugMode.Halt();
            Expect.True(DebugMode.IsOn, "IsOn after Halt while off");
            Expect.Equal(1, debug.InputListens, "input listens after Halt while off");
            Expect.True(Runtime.Clock.IsHalted, "the clock after Halt while off");
            Expect.True(Paused(world, unit), "the unit after Halt while off");
            Expect.Equal(printed + 2, world.PrintedTexts.Count, "lines printed: the mode's and the halt's");
            Expect.True(world.PrintedTexts[printed].StartsWith("Debug mode on"), "the first line is -debug's");
            DebugMode.Halt();
            Expect.Equal(printed + 2, world.PrintedTexts.Count, "lines printed by a second Halt");
            debug.Ctrl(0, DebugKey.H);
            Expect.True(!Runtime.Clock.IsHalted, "the clock after Ctrl + H");
            Expect.True(!Paused(world, unit), "the unit after Ctrl + H");
        }

        /// <summary>A held Ctrl + H, its down event repeated with no up between, halts once; after the up, the next down resumes.</summary>
        public static void HeldHTogglesOnce(FakeWorld world)
        {
            var debug = On(world);
            debug.Key(0, DebugKey.Control, 0, true);
            debug.Key(0, DebugKey.H, 2, true);
            Expect.True(Runtime.Clock.IsHalted, "the clock after the first H down");
            debug.Key(0, DebugKey.H, 2, true);
            Expect.True(Runtime.Clock.IsHalted, "the clock after a repeat");
            debug.Key(0, DebugKey.H, 2, true);
            Expect.True(Runtime.Clock.IsHalted, "the clock after a second repeat");
            debug.Key(1, DebugKey.H, 2, true);
            Expect.True(!Runtime.Clock.IsHalted, "the clock after another slot's H down");
            debug.Key(1, DebugKey.H, 0, false);
            debug.Key(1, DebugKey.H, 2, true);
            Expect.True(Runtime.Clock.IsHalted, "the clock after the other slot's next H down");
            debug.Key(0, DebugKey.H, 0, false);
            debug.Key(0, DebugKey.H, 2, true);
            Expect.True(!Runtime.Clock.IsHalted, "the clock after the first slot's up and down");
        }

        /// <summary>A held unit removed while held: the resume lets go of the others, makes no call on it, and reports no error.</summary>
        public static void HeldUnitRemoved(FakeWorld world)
        {
            var a = UnitAt(world, 0.0f, 0.0f);
            var removed = UnitAt(world, 100.0f, 0.0f);
            var b = UnitAt(world, 200.0f, 0.0f);
            var debug = On(world);
            debug.Ctrl(0, DebugKey.H);
            Expect.True(Paused(world, removed), "the unit is held");
            removed.Remove();
            var logged = world.Logged.Count;
            debug.Ctrl(0, DebugKey.H);
            Expect.True(!Runtime.Clock.IsHalted, "the clock after Ctrl + H");
            Expect.True(!Paused(world, a) && !Paused(world, b), "the other units after the resume");
            Expect.Equal(logged, world.Logged.Count, "errors logged by the resume");
            var callsOnRemoved = 0;
            for (var i = 0; i < debug.PausedOrLetGo.Count; i++)
            {
                if (debug.PausedOrLetGo[i] == removed.Handle)
                {
                    callsOnRemoved++;
                }
            }

            Expect.Equal(1, callsOnRemoved, "pause calls on the removed unit: its hold only");
        }

        /// <summary>DebugMode's IsOn, Selected and IsSelected follow the session, Select selects, and Halt halts with units held; while off, Select does nothing.</summary>
        public static void Facade(FakeWorld world)
        {
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 500.0f, 0.0f);
            var debug = FakeDebugWorld.Install(world);
            Expect.True(!DebugMode.IsOn, "IsOn while off");
            DebugMode.Select(a);
            Expect.Same(null, DebugMode.Selected, "Selected after Select while off");
            debug.Chat(0, "-debug");
            Expect.True(DebugMode.IsOn, "IsOn while on");
            debug.CtrlClick(0, 0.0f, 0.0f);
            Expect.Same(a, DebugMode.Selected, "Selected after a Ctrl + click");
            Expect.True(DebugMode.IsSelected(a), "IsSelected of the unit clicked");
            Expect.True(!DebugMode.IsSelected(b), "IsSelected of the other unit");
            DebugMode.Select(b);
            Expect.Same(b, DebugMode.Selected, "Selected after Select");
            Expect.True(DebugMode.IsSelected(b) && !DebugMode.IsSelected(a), "IsSelected after Select");
            DebugMode.Halt();
            Expect.True(Runtime.Clock.IsHalted, "the clock after Halt");
            Expect.True(Paused(world, a) && Paused(world, b), "the units after Halt");
            DebugMode.Halt();
            Expect.True(Runtime.Clock.IsHalted, "the clock after a second Halt");
            DebugMode.Select(null);
            Expect.Same(null, DebugMode.Selected, "Selected after Select(null)");
        }

        /// <summary>
        /// A breakpoint whose condition always holds, in a timer every step: the first halt turns the mode on; after Ctrl + H
        /// the next step halts again; -debug off escapes it, the mode staying off and the clock running; -debug on again lets it
        /// halt once more.
        /// </summary>
        public static void BreakpointEscapes(FakeWorld world)
        {
            var unit = UnitAt(world, 0.0f, 0.0f);
            var debug = FakeDebugWorld.Install(world);
            Timer.Every(Step, () => { DebugMode.Halt(); });
            debug.Advance(Step);
            Expect.True(DebugSession.IsOn, "the mode after the first halt from code");
            Expect.True(Runtime.Clock.IsHalted, "the clock after the first halt from code");
            debug.Ctrl(0, DebugKey.H);
            Expect.True(!Runtime.Clock.IsHalted, "the clock after Ctrl + H");
            debug.Advance(Step);
            Expect.True(Runtime.Clock.IsHalted, "the clock a step after Ctrl + H: the breakpoint halts again");
            debug.Chat(0, "-debug");
            Expect.True(!DebugSession.IsOn, "the mode after -debug off");
            Expect.True(!Runtime.Clock.IsHalted, "the clock after -debug off");
            var before = Runtime.World.Now();
            debug.Advance(1.0f);
            Expect.True(!DebugSession.IsOn, "the mode a second after -debug off");
            Expect.True(!Runtime.Clock.IsHalted, "the clock a second after -debug off");
            Expect.True(!Paused(world, unit), "the unit a second after -debug off");
            Expect.Near(before + 1.0f, Runtime.World.Now(), "Moonwell's time a second after -debug off");
            debug.Chat(0, "-debug");
            debug.Advance(Step);
            Expect.True(Runtime.Clock.IsHalted, "the clock a step after -debug on again");
        }

        /// <summary>Right Ctrl does as left Ctrl: with it down a press selects; with left let go and right still down, too; with both up, nothing.</summary>
        public static void RightCtrlSelects(FakeWorld world)
        {
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 600.0f, 0.0f);
            var c = UnitAt(world, 1200.0f, 0.0f);
            var debug = On(world);
            debug.Key(0, DebugKey.RightControl, 0, true);
            debug.Press(0, 0.0f, 0.0f);
            Expect.Same(a, DebugSession.Selected, "the unit selected with right Ctrl down");
            debug.Key(0, DebugKey.Control, 2, true);
            debug.Key(0, DebugKey.Control, 0, false);
            debug.Press(0, 600.0f, 0.0f);
            Expect.Same(b, DebugSession.Selected, "the unit selected with left Ctrl let go and right Ctrl still down");
            debug.Key(0, DebugKey.RightControl, 0, false);
            debug.Press(0, 1200.0f, 0.0f);
            Expect.Same(b, DebugSession.Selected, "the selection after a press with both Ctrl keys up");
            debug.Key(0, DebugKey.RightControl, 0, true);
            debug.Key(0, DebugKey.H, 2, true);
            Expect.True(Runtime.Clock.IsHalted, "the clock after right Ctrl + H");
            debug.Press(0, 1200.0f, 0.0f);
            Expect.Same(c, DebugSession.Selected, "the unit selected with right Ctrl down while halted");
        }

        /// <summary>-step alone takes one step and prints "1 step.", as -step 1 does; running, it prints that the clock is not halted.</summary>
        public static void StepBare(FakeWorld world)
        {
            var debug = On(world);
            var printed = world.PrintedTexts.Count;
            debug.Chat(0, "-step");
            Expect.Equal(printed + 1, world.PrintedTexts.Count, "lines printed by -step while running");
            Expect.Equal("Not halted: Ctrl + H halts.", world.PrintedTexts[printed], "the line of -step while running");
            debug.Ctrl(0, DebugKey.H, shift: true);
            var before = Runtime.World.Now();
            printed = world.PrintedTexts.Count;
            debug.Chat(0, "-step");
            Expect.Near(before + Step, Runtime.World.Now(), "Moonwell's time after -step");
            Expect.Equal(printed + 1, world.PrintedTexts.Count, "lines printed by -step");
            Expect.Equal("1 step.", world.PrintedTexts[printed], "the line of -step");
        }

        /// <summary>-speed alone prints the six speeds in one line and changes nothing.</summary>
        public static void SpeedBare(FakeWorld world)
        {
            var debug = On(world);
            debug.Chat(0, "-speed 2");
            var printed = world.PrintedTexts.Count;
            debug.Chat(0, "-speed");
            Expect.Equal(printed + 1, world.PrintedTexts.Count, "lines printed by -speed");
            Expect.Equal("Speeds: 1/4, 1/2, 1, 2, 4, 8.", world.PrintedTexts[printed], "the line of -speed");
            Expect.Near(2.0f, Runtime.Clock.Speed, "the speed after -speed");
        }

        /// <summary>A selected unit removed: DebugMode.Selected reads null and IsSelected false at once, before the panel's next run.</summary>
        public static void EndedSelectedReadsNull(FakeWorld world)
        {
            var unit = UnitAt(world, 0.0f, 0.0f);
            var debug = On(world);
            debug.CtrlClick(0, 0.0f, 0.0f);
            Expect.Same(unit, DebugMode.Selected, "Selected after the click");
            unit.Remove();
            Expect.Same(null, DebugMode.Selected, "Selected after the unit is removed");
            Expect.True(!DebugMode.IsSelected(unit), "IsSelected of the removed unit");
            debug.Advance(Step);
            Expect.Same(null, DebugMode.Selected, "Selected after the panel's next run");
        }
    }
#endif
}
