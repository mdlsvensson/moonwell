#if DEBUG
using System;
using System.Collections.Generic;
using Moonwell.Hosting;

namespace Moonwell
{
    /// <summary>
    /// The debug mode in a development build: -debug turns it on and off; while on, Ctrl + click selects, Ctrl + H halts
    /// Moonwell's clock and holds the units, Ctrl + N steps, Ctrl + "-" and "=" change the speed. Every input is a synced
    /// event, so every machine does the same at the same moment.
    /// </summary>
    internal static class DebugSession
    {
        /// <summary>How near a click an object can be selected.</summary>
        private const float ClickRadius = 150.0f;

        /// <summary>How near the last click a click is at the same spot, and takes the next nearest object.</summary>
        private const float SameSpot = 32.0f;

        /// <summary>The most steps one -step takes.</summary>
        private const int MostSteps = 320;

        /// <summary>The speeds, slowest first.</summary>
        private static readonly float[] Speeds = new float[] { 0.25f, 0.5f, 1.0f, 2.0f, 4.0f, 8.0f };

        /// <summary>The speeds as the lines name them, beside Speeds.</summary>
        private static readonly string[] SpeedNames = new string[] { "1/4", "1/2", "1", "2", "4", "8" };

        /// <summary>The place of speed 1 in Speeds.</summary>
        private const int NormalSpeed = 2;

        /// <summary>The debug world; null until Install.</summary>
        private static IDebugWorld? world;

        /// <summary>The units a halt holds; made by Install.</summary>
        private static HeldUnits? held;

        /// <summary>Whether the keys and the mouse are listened to; they are from the first -debug on.</summary>
        private static bool listening;

        /// <summary>Whether each player slot's left Ctrl is down.</summary>
        private static bool[] controlDown = new bool[24];

        /// <summary>Whether each player slot's H is down: a held key repeats its down event, and only the first toggles the halt.</summary>
        private static bool[] hDown = new bool[24];

        /// <summary>Whether a click has been taken, and where the last was.</summary>
        private static bool clicked;

        /// <summary>The x of the last click.</summary>
        private static float lastX;

        /// <summary>The y of the last click.</summary>
        private static float lastY;

        /// <summary>Whether the halt holds the units (Ctrl + H) rather than leaving them running (Ctrl + Shift + H).</summary>
        private static bool holdsUnits;

        /// <summary>Whether the units are held now: during a held halt, but not while a step lets them go.</summary>
        private static bool holding;

        /// <summary>The steps of a -step still to take after the one running.</summary>
        private static int stepsLeft;

        /// <summary>The number of the last held step started; a step's end that is not the last's does nothing.</summary>
        private static int stepNumber;

        /// <summary>The place of the speed in Speeds.</summary>
        private static int speedIndex = NormalSpeed;

        /// <summary>Whether the debug mode is on.</summary>
        internal static bool IsOn { get; private set; }

        /// <summary>The object selected; null when none.</summary>
        internal static Actor? Selected { get; private set; }

        /// <summary>Whether a held step is running: the units are let go for one step of game time, and another step waits for it.</summary>
        internal static bool StepRunning { get; private set; }

        /// <summary>Whether the halt holds the units (Ctrl + H) rather than leaving them running (Ctrl + Shift + H).</summary>
        internal static bool HoldsUnits
        {
            get
            {
                return holdsUnits;
            }
        }

        /// <summary>The speed as the panel names it: "1/4", "1/2", "1", "2", "4" or "8".</summary>
        internal static string SpeedName
        {
            get
            {
                return SpeedNames[speedIndex];
            }
        }

        /// <summary>Makes a debug world the session's, and listens to the chat for -debug; Runtime.Start calls it after Reset.</summary>
        internal static void Install(IDebugWorld debugWorld)
        {
            world = debugWorld;
            held = new HeldUnits(debugWorld);
            debugWorld.ListenChat(Heard);
        }

        /// <summary>Forgets the debug world and the session's state; Runtime.Reset calls it.</summary>
        internal static void Reset()
        {
            world = null;
            held = null;
            listening = false;
            controlDown = new bool[24];
            hDown = new bool[24];
            clicked = false;
            holdsUnits = false;
            holding = false;
            stepsLeft = 0;
            stepNumber++;
            speedIndex = NormalSpeed;
            IsOn = false;
            Selected = null;
            StepRunning = false;
        }

        /// <summary>Real seconds of this machine's processor (os.clock) from the debug world, for timing; 0 when none is installed.</summary>
        internal static float Clock()
        {
            return world != null ? world.Clock() : 0.0f;
        }

        /// <summary>Turns the mode on, listening to the keys and the mouse the first time; or off, resuming at speed 1, letting go of held units and clearing the selection.</summary>
        internal static void Toggle()
        {
            if (world == null)
            {
                return;
            }

            if (!IsOn)
            {
                IsOn = true;
                if (!listening)
                {
                    listening = true;
                    world.ListenInput(KeyHeard, LeftPressed);
                }

                Game.Print("Debug mode on: Ctrl + click selects, Ctrl + H halts, Ctrl + N steps, Ctrl + - and = change the speed.");
                DebugPanel.TurnedOn(world);
                return;
            }

            ResumeClock();
            ChangeSpeed(NormalSpeed);
            Selected = null;
            IsOn = false;
            DebugPanel.TurnedOff();
            Game.Print("Debug mode off.");
        }

        /// <summary>Selects an object, or nothing with null, and refreshes the panel; nothing while the mode is off.</summary>
        internal static void Select(Actor? actor)
        {
            if (!IsOn)
            {
                return;
            }

            Selected = actor;
            DebugPanel.SelectionChanged();
        }

        /// <summary>Lets go of the selected object once it has ended; the panel calls it, and shows the object's line instead.</summary>
        internal static void SelectedEnded()
        {
            Selected = null;
        }

        /// <summary>Halts Moonwell's clock, holding every unit when asked; nothing while the mode is off or already halted.</summary>
        internal static void Halt(bool holdUnits)
        {
            if (world == null || !IsOn || Runtime.Clock.IsHalted)
            {
                return;
            }

            Runtime.Clock.Halt(world.GameNow());
            Runtime.Scheduler.Rearm();
            holdsUnits = holdUnits;
            if (holdUnits)
            {
                held!.Hold();
                holding = true;
                Game.Print("Halted, units held: Ctrl + N steps, Ctrl + H resumes.");
            }
            else
            {
                Game.Print("Halted, units running: Ctrl + N steps, Ctrl + Shift + H resumes.");
            }

            DebugPanel.Refresh();
        }

        /// <summary>A halt from code (DebugMode.Halt), like a breakpoint: turns the mode on first when it is off, as -debug does, then halts with every unit held; nothing when already halted.</summary>
        internal static void HaltFromCode()
        {
            if (world == null || Runtime.Clock.IsHalted)
            {
                return;
            }

            if (!IsOn)
            {
                Toggle();
            }

            Halt(true);
        }

        /// <summary>Lets Moonwell's clock run on, and lets go of the held units; nothing while the mode is off or not halted.</summary>
        internal static void Resume()
        {
            if (!IsOn || !Runtime.Clock.IsHalted)
            {
                return;
            }

            ResumeClock();
            Game.Print("Resumed.");
            DebugPanel.Refresh();
        }

        /// <summary>
        /// One step while halted: with units running, Moonwell's time moves one step at once and what is due runs; with units
        /// held, they are let go for one step of game time, then Moonwell's time moves one step and they are held again.
        /// Nothing while off, running, or while a held step runs.
        /// </summary>
        internal static void Step()
        {
            if (world == null || !IsOn || !Runtime.Clock.IsHalted || StepRunning)
            {
                return;
            }

            if (!holdsUnits)
            {
                FreeStep();
                DebugPanel.Refresh();
                return;
            }

            StepRunning = true;
            stepNumber++;
            var number = stepNumber;
            holding = false;
            held!.LetGo();
            world.After(DebugViews.StepLength, () => StepEnded(number));
        }

        /// <summary>Sets the speed to one of ¼, ½, 1, 2, 4 and 8; any other prints a line and changes nothing. Nothing while off.</summary>
        internal static void SetSpeed(float speed)
        {
            if (!IsOn)
            {
                return;
            }

            for (var i = 0; i < Speeds.Length; i++)
            {
                if (Speeds[i] == speed)
                {
                    ChangeSpeed(i);
                    Game.Print("Speed " + SpeedNames[i] + ".");
                    DebugPanel.Refresh();
                    return;
                }
            }

            Game.Print("Speeds: 1/4, 1/2, 1, 2, 4, 8.");
        }

        /// <summary>Holds a unit taken in while the units are held, as a unit made while held is not paused by the game; KnownUnits.TakeIn calls it.</summary>
        internal static void UnitTakenIn(Unit unit)
        {
            if (holding)
            {
                held!.Add(unit);
            }
        }

        /// <summary>One step with units running: Moonwell's time moves one step at once, and what is due runs.</summary>
        private static void FreeStep()
        {
            Runtime.Clock.Advance(DebugViews.StepLength);
            Runtime.Wake();
        }

        /// <summary>The end of a held step: Moonwell's time moves one step, what is due runs, the units are held again, and a -step goes on. A step a resume or -debug ended does nothing.</summary>
        private static void StepEnded(int number)
        {
            if (number != stepNumber || !StepRunning)
            {
                return;
            }

            Runtime.Clock.Advance(DebugViews.StepLength);
            Runtime.Wake();
            held!.Hold();
            holding = true;
            StepRunning = false;
            DebugPanel.Refresh();
            if (stepsLeft > 0)
            {
                stepsLeft--;
                Step();
            }
        }

        /// <summary>Resumes the clock at the game's time, re-arms the wake, and lets go of the held units; a running step ends.</summary>
        private static void ResumeClock()
        {
            if (world == null || !Runtime.Clock.IsHalted)
            {
                return;
            }

            Runtime.Clock.Resume(world.GameNow());
            Runtime.Scheduler.Rearm();
            held!.LetGo();
            holding = false;
            holdsUnits = false;
            StepRunning = false;
            stepsLeft = 0;
            stepNumber++;
        }

        /// <summary>Sets the clock's speed to the one at a place in Speeds, and re-arms the wake.</summary>
        private static void ChangeSpeed(int index)
        {
            speedIndex = index;
            Runtime.Clock.SetSpeed(Speeds[index], world!.GameNow());
            Runtime.Scheduler.Rearm();
        }

        /// <summary>A chat line with "-" in it from a slot: -debug always; -step, -speed and -stats while on. Anything else is left alone.</summary>
        private static void Heard(int slot, string text)
        {
            var line = text.Trim().ToLower();
            if (line == "-debug")
            {
                Toggle();
                return;
            }

            if (!IsOn)
            {
                return;
            }

            if (line == "-stats")
            {
                DebugPanel.ToggleStats();
            }
            else if (line.StartsWith("-step "))
            {
                StepCommand(line.Substring(6));
            }
            else if (line.StartsWith("-speed "))
            {
                if (Number(line.Substring(7), out var speed))
                {
                    SetSpeed(speed);
                }
                else
                {
                    Game.Print("Speeds: 1/4, 1/2, 1, 2, 4, 8.");
                }
            }
        }

        /// <summary>-step with its count: that many steps, one after another, while halted.</summary>
        private static void StepCommand(string count)
        {
            if (!Number(count, out var steps) || steps < 1.0f || steps > MostSteps || steps != (int)steps)
            {
                Game.Print("Steps: 1 to " + MostSteps + ".");
                return;
            }

            if (!Runtime.Clock.IsHalted)
            {
                Game.Print("Not halted: Ctrl + H halts.");
                return;
            }

            if (StepRunning)
            {
                return;
            }

            var whole = (int)steps;
            Game.Print(whole == 1 ? "1 step." : whole + " steps.");
            if (!holdsUnits)
            {
                // The steps run at once, so the panel is refreshed once, after the last.
                for (var i = 0; i < whole; i++)
                {
                    FreeStep();
                }

                DebugPanel.Refresh();
                return;
            }

            stepsLeft = whole - 1;
            Step();
        }

        /// <summary>A key event from a slot: Ctrl's and H's state always; while on, H halts or resumes (a repeat of a held H does nothing), N steps, "-" and "=" change the speed.</summary>
        private static void KeyHeard(int slot, int key, int meta, bool down)
        {
            if (key == DebugKey.Control)
            {
                if (slot >= 0 && slot < controlDown.Length)
                {
                    controlDown[slot] = down;
                }

                return;
            }

            if (key == DebugKey.H && slot >= 0 && slot < hDown.Length)
            {
                var repeat = down && hDown[slot];
                hDown[slot] = down;
                if (repeat)
                {
                    return;
                }
            }

            if (!IsOn || !down)
            {
                return;
            }

            if (key == DebugKey.H)
            {
                if (Runtime.Clock.IsHalted)
                {
                    Resume();
                }
                else
                {
                    Halt(meta != 3);
                }
            }
            else if (key == DebugKey.N)
            {
                Step();
            }
            else if (key == DebugKey.Minus)
            {
                if (speedIndex > 0)
                {
                    SetSpeed(Speeds[speedIndex - 1]);
                }
            }
            else if (key == DebugKey.Plus)
            {
                if (speedIndex < Speeds.Length - 1)
                {
                    SetSpeed(Speeds[speedIndex + 1]);
                }
            }
        }

        /// <summary>A left press from a slot at a ground point: with that slot's Ctrl down while on, selects the nearest object within 150, the next nearest at the same spot, or nothing.</summary>
        private static void LeftPressed(int slot, float x, float y)
        {
            if (!IsOn || slot < 0 || slot >= controlDown.Length || !controlDown[slot])
            {
                return;
            }

            var sameSpot = clicked && (x - lastX) * (x - lastX) + (y - lastY) * (y - lastY) <= SameSpot * SameSpot;
            clicked = true;
            lastX = x;
            lastY = y;
            var found = new List<Actor>();
            var distances = new List<float>();
            Nearby(x, y, found, distances);
            if (found.Count == 0)
            {
                Selected = null;
                DebugPanel.SelectionChanged();
                return;
            }

            var pick = 0;
            if (sameSpot && Selected != null)
            {
                var at = found.IndexOf(Selected);
                if (at >= 0)
                {
                    pick = (at + 1) % found.Count;
                }
            }

            Selected = found[pick];
            DebugPanel.SelectionChanged();
        }

        /// <summary>The objects within the click radius of a point, nearest first, ties in the order found: the live units, then each view's, with their squared distances beside them.</summary>
        private static void Nearby(float x, float y, List<Actor> found, List<float> distances)
        {
            var candidates = new List<Actor>();
            var live = KnownUnits.Live;
            for (var i = 0; i < live.Count; i++)
            {
                candidates.Add(live[i]);
            }

            var views = DebugViews.All;
            for (var i = 0; i < views.Count; i++)
            {
                views[i].Candidates(x, y, ClickRadius, candidates);
            }

            for (var i = 0; i < candidates.Count; i++)
            {
                var actor = candidates[i];
                if (actor.HasEnded || found.Contains(actor))
                {
                    continue;
                }

                // Its own statement: an out variable declared in a condition after || reaches no further in Lua.
                var placed = PlaceOf(actor, out var ax, out var ay);
                if (!placed)
                {
                    continue;
                }

                var distance = (ax - x) * (ax - x) + (ay - y) * (ay - y);
                if (distance > ClickRadius * ClickRadius)
                {
                    continue;
                }

                var at = found.Count;
                while (at > 0 && distances[at - 1] > distance)
                {
                    at--;
                }

                found.Insert(at, actor);
                distances.Insert(at, distance);
            }
        }

        /// <summary>Where an object stands: a unit's, item's or destructable's place, or the place a view gives; false when none knows it.</summary>
        internal static bool PlaceOf(Actor actor, out float x, out float y)
        {
            if (actor is Unit unit)
            {
                x = unit.X;
                y = unit.Y;
                return !unit.IsGone;
            }

            if (actor is Item item)
            {
                x = item.X;
                y = item.Y;
                return !item.IsGone;
            }

            if (actor is Destructable destructable)
            {
                x = destructable.X;
                y = destructable.Y;
                return !destructable.IsGone;
            }

            var views = DebugViews.All;
            for (var i = 0; i < views.Count; i++)
            {
                if (views[i].Place(actor, out x, out y))
                {
                    return true;
                }
            }

            x = 0.0f;
            y = 0.0f;
            return false;
        }

        /// <summary>
        /// A number of chat text: digits with a decimal point or a fraction ("0.25", ".5", "1/4"); false for anything else.
        /// Read by hand, so the game's Lua and .NET read it the same.
        /// </summary>
        private static bool Number(string text, out float value)
        {
            value = 0.0f;
            var slash = text.IndexOf('/');
            if (slash >= 0)
            {
                // Two statements: an out variable declared in a condition after || reaches no further in Lua.
                var readTop = Number(text.Substring(0, slash), out var top);
                var readBottom = Number(text.Substring(slash + 1), out var bottom);
                if (!readTop || !readBottom || bottom == 0.0f)
                {
                    return false;
                }

                value = top / bottom;
                return true;
            }

            // The digits as one whole number, divided by 10 for each digit after the point: 0.25 is 25 / 100, exact in both.
            var number = 0.0f;
            var divisor = 1.0f;
            var digits = 0;
            var point = false;
            for (var i = 0; i < text.Length; i++)
            {
                var c = text[i];
                if (c == '.' && !point)
                {
                    point = true;
                }
                else if (c >= '0' && c <= '9')
                {
                    digits++;
                    number = number * 10.0f + (c - '0');
                    if (point)
                    {
                        divisor = divisor * 10.0f;
                    }
                }
                else
                {
                    return false;
                }
            }

            value = number / divisor;
            return digits > 0;
        }
    }
}
#endif
