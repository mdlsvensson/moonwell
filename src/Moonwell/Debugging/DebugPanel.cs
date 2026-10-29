#if DEBUG
using System;
using System.Collections.Generic;
using Moonwell.Hosting;

namespace Moonwell
{
    /// <summary>
    /// The debug mode's panel and drawings: the status, the selected object's lines, the views' lines and the map's own
    /// (IDebugShown), and with -stats the statistics; on the map, a marker on the selected object and the views' drawings.
    /// A refresher on game time (Moonwell's timers stop when halted) draws every 1/32 s while running or halted with units
    /// running, and sets the text every 8th time; halted with units held it does nothing, and each step, selection, halt
    /// and resume refreshes at once.
    /// </summary>
    internal static class DebugPanel
    {
        /// <summary>The most lines the panel shows; past it the last says how many more there are.</summary>
        private const int MostLines = 30;

        /// <summary>The game seconds between two runs of the refresher.</summary>
        private const float RefreshEvery = 1.0f / 32.0f;

        /// <summary>The refresher sets the text every this many runs: 4 times a second.</summary>
        private const int TextEvery = 8;

        /// <summary>The debug world; null until the mode is first turned on.</summary>
        private static IDebugWorld? world;

        /// <summary>The drawing of the debug world; made with it.</summary>
        private static DebugDraw? draw;

        /// <summary>The refresher's number: turning the mode off or Reset raise it, and a refresher whose number is not the current one stops.</summary>
        private static int generation;

        /// <summary>The refresher's runs while running, for setting the text every 8th.</summary>
        private static int runs;

        /// <summary>The line of the selected object that ended, "Footman 1 ended", until another selection; null when none.</summary>
        private static string? endedLine;

        /// <summary>Whether -stats shows the statistics.</summary>
        private static bool statsShown;

        /// <summary>The objects the panel has named, in the order it first named them; searched by reference, as a map's class may hold static fields.</summary>
        private static List<Scope> named = new List<Scope>();

        /// <summary>The number of each object in named, counted within its class.</summary>
        private static List<int> numbers = new List<int>();

        /// <summary>Forgets the world, the drawings, the numbers and the statistics' switch, and stops the refresher; Runtime.Reset calls it.</summary>
        internal static void Reset()
        {
            world = null;
            draw = null;
            generation++;
            runs = 0;
            endedLine = null;
            statsShown = false;
            named = new List<Scope>();
            numbers = new List<int>();
        }

        /// <summary>The mode is turned on: the panel is shown (its frames made the first time), the refresher starts, and the panel is refreshed.</summary>
        internal static void TurnedOn(IDebugWorld debugWorld)
        {
            if (world != debugWorld)
            {
                world = debugWorld;
                draw = new DebugDraw(debugWorld);
            }

            debugWorld.ShowPanel(true);
            generation++;
            runs = 0;
            var number = generation;
            debugWorld.After(RefreshEvery, () => Run(number));
            Refresh();
        }

        /// <summary>The mode is turned off: the refresher stops, every line and marker is removed, and the panel is hidden.</summary>
        internal static void TurnedOff()
        {
            generation++;
            endedLine = null;
            if (world == null)
            {
                return;
            }

            draw!.Clear();
            world.ShowPanel(false);
        }

        /// <summary>The selection changed: an ended object's line goes, and the panel is refreshed.</summary>
        internal static void SelectionChanged()
        {
            endedLine = null;
            Refresh();
        }

        /// <summary>-stats: shows the statistics and writes them to the log, or hides them.</summary>
        internal static void ToggleStats()
        {
            if (world == null)
            {
                return;
            }

            statsShown = !statsShown;
            Game.Print(statsShown ? "Statistics shown." : "Statistics hidden.");
            if (statsShown)
            {
                var lines = new DebugLines();
                AddStats(lines);
                for (var i = 0; i < lines.Count; i++)
                {
                    world.Log(lines.At(i));
                }
            }

            Refresh();
        }

        /// <summary>Draws and sets the text at once; nothing while the mode is off.</summary>
        internal static void Refresh()
        {
            if (world == null || !DebugSession.IsOn)
            {
                return;
            }

            Draw();
            world.SetPanelText(Text());
        }

        /// <summary>The name the panel gives an object: "Game", "Player 1", or its class and its number within the class, given the first time it is named.</summary>
        internal static string NameOf(Scope obj)
        {
            if (obj is Game)
            {
                return "Game";
            }

            if (obj is Player player)
            {
                return PlayerName(player);
            }

            return Names.NameOf(obj) + " " + NumberOf(obj);
        }

        /// <summary>A whole number with a comma between thousands: "1,204".</summary>
        internal static string Count(int value)
        {
            if (value < 0)
            {
                return "-" + Count(-value);
            }

            if (value < 1000)
            {
                return "" + value;
            }

            var rest = value % 1000;
            var zeros = rest < 10 ? "00" : (rest < 100 ? "0" : "");
            return Count(value / 1000) + "," + zeros + rest;
        }

        /// <summary>A number rounded to a whole one, with a comma between thousands: "-256", "1,235".</summary>
        internal static string Whole(float value)
        {
            return Count(Round(value));
        }

        /// <summary>A number rounded to tenths: "37.6", "0.1".</summary>
        internal static string Tenths(float value)
        {
            var tenths = Round(value * 10.0f);
            if (tenths < 0)
            {
                return "-" + Count(-tenths / 10) + "." + (-tenths % 10);
            }

            return Count(tenths / 10) + "." + (tenths % 10);
        }

        /// <summary>
        /// One run of the refresher, which asks for the next first: while running it closes a statistics step, draws, and sets
        /// the text every 8th run or when the selected object ended; halted with units running, it does the same with no
        /// statistics step, as the units still move; halted with units held, it refreshes only for a selected object that
        /// ended.
        /// </summary>
        private static void Run(int number)
        {
            if (number != generation || world == null)
            {
                return;
            }

            world.After(RefreshEvery, () => Run(number));
            var selected = DebugSession.Selected;
            var ended = selected != null && selected.HasEnded;
            var halted = Runtime.Clock.IsHalted;
            if (halted && DebugSession.HoldsUnits)
            {
                if (ended)
                {
                    Refresh();
                }

                return;
            }

            if (!halted)
            {
                DebugStats.Tick();
            }

            runs++;
            Draw();
            if (ended || runs % TextEvery == 0)
            {
                world.SetPanelText(Text());
            }
        }

        /// <summary>The selected object, or null; a selected object that ended is let go here, and leaves its line.</summary>
        private static Actor? Selected()
        {
            var selected = DebugSession.Selected;
            if (selected != null && selected.HasEnded)
            {
                endedLine = NameOf(selected) + " ended";
                DebugSession.SelectedEnded();
                return null;
            }

            return selected;
        }

        /// <summary>A frame of drawing: the marker on the selected object, and each view's drawings of it; nothing selected removes every drawing.</summary>
        private static void Draw()
        {
            var selected = Selected();
            draw!.Begin();
            if (selected != null)
            {
                if (DebugSession.PlaceOf(selected, out var x, out var y))
                {
                    draw.Marker(x, y);
                }

                var views = DebugViews.All;
                for (var i = 0; i < views.Count; i++)
                {
                    try
                    {
                        views[i].Draw(selected, draw);
                    }
                    catch (Exception error)
                    {
                        Errors.Report("Moonwell.Debug", error);
                    }
                }
            }

            draw.End();
        }

        /// <summary>The panel's text: the status; the selected object's line, the views' lines and the IDebugShown lines, or an ended object's line; with -stats the statistics; at most 30 lines.</summary>
        private static string Text()
        {
            var lines = new DebugLines();
            lines.Add(Status());
            var selected = Selected();
            if (selected != null)
            {
                lines.Add(ObjectLine(selected));
                var views = DebugViews.All;
                for (var i = 0; i < views.Count; i++)
                {
                    try
                    {
                        views[i].Lines(selected, lines);
                    }
                    catch (Exception error)
                    {
                        Errors.Report("Moonwell.Debug", error);
                    }
                }

                AddShown(selected, lines);
            }
            else if (endedLine != null)
            {
                lines.Add(endedLine);
            }

            if (statsShown)
            {
                AddStats(lines);
            }

            if (lines.Count <= MostLines)
            {
                return lines.Text;
            }

            var text = lines.At(0);
            for (var i = 1; i < MostLines - 1; i++)
            {
                text = text + "\n" + lines.At(i);
            }

            return text + "\nand " + Count(lines.Count - (MostLines - 1)) + " more";
        }

        /// <summary>"DEBUG  halted · step 1,204 (37.6 s) · speed 1 · units held": the step is Moonwell's time in whole steps, and the seconds Moonwell's time.</summary>
        private static string Status()
        {
            var now = Runtime.World.Now();
            var halted = Runtime.Clock.IsHalted;
            // A thousandth of a step is forgiven, as a time that is a whole number of steps can come out a little under in a float.
            var step = (int)(now / DebugViews.StepLength + 0.001f);
            var line = "DEBUG  " + (halted ? "halted" : "running") + " · step " + Count(step) + " (" + Tenths(now) + " s) · speed " + DebugSession.SpeedName;
            if (halted)
            {
                line = line + (DebugSession.HoldsUnits ? " · units held" : " · units running");
            }

            return line;
        }

        /// <summary>"Footman 1 (unit) at 512, -256 · Player 1 · in Inferno 3": what is known of the object's place, player and scope.</summary>
        private static string ObjectLine(Actor actor)
        {
            var line = NameOf(actor) + " (" + KindOf(actor) + ")";
            if (DebugSession.PlaceOf(actor, out var x, out var y))
            {
                line = line + " at " + Whole(x) + ", " + Whole(y);
            }

            if (actor is Unit unit)
            {
                line = line + " · " + PlayerName(unit.Player);
            }
            else if (actor is Item item)
            {
                line = line + " · " + PlayerName(item.Player);
            }

            var parent = actor.Parent;
            if (parent != null)
            {
                line = line + " · in " + NameOf(parent);
            }

            return line;
        }

        /// <summary>The map's own lines: the object's ShowDebug, then each component's in add order; one that throws is reported as "Class.ShowDebug", and the panel goes on.</summary>
        private static void AddShown(Actor actor, DebugLines lines)
        {
            if (actor is IDebugShown own)
            {
                Ask(own, actor, lines);
            }

            var count = actor.BeginDispatch();
            for (var i = 0; i < count; i++)
            {
                var component = actor.ComponentAt(i);
                if (component is IDebugShown shown)
                {
                    Ask(shown, component, lines);
                }
            }

            actor.EndDispatch();
        }

        /// <summary>Calls one ShowDebug, reporting an error it throws as "Class.ShowDebug".</summary>
        private static void Ask(IDebugShown shown, object obj, DebugLines lines)
        {
            try
            {
                shown.ShowDebug(lines);
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(obj) + ".ShowDebug", error);
            }
        }

        /// <summary>The statistics: each view's, then the timers' line.</summary>
        private static void AddStats(DebugLines lines)
        {
            var views = DebugViews.All;
            for (var i = 0; i < views.Count; i++)
            {
                try
                {
                    views[i].Stats(lines);
                }
                catch (Exception error)
                {
                    Errors.Report("Moonwell.Debug", error);
                }
            }

            lines.Add(DebugStats.TimersLine());
        }

        /// <summary>The number of an object within its class: the one it was given, or the next of its class, given now.</summary>
        private static int NumberOf(Scope obj)
        {
            var at = named.IndexOf(obj);
            if (at >= 0)
            {
                return numbers[at];
            }

            var kind = Names.ClassOf(obj);
            var number = 1;
            for (var i = 0; i < named.Count; i++)
            {
                if (Names.ClassOf(named[i]) == kind)
                {
                    number++;
                }
            }

            named.Add(obj);
            numbers.Add(number);
            return number;
        }

        /// <summary>The kind of object an actor is, as the panel names it; an actor of no kind core knows is a gizmo, the interactions package's.</summary>
        private static string KindOf(Actor actor)
        {
            if (actor is Unit)
            {
                return "unit";
            }

            if (actor is Item)
            {
                return "item";
            }

            if (actor is Destructable)
            {
                return "destructable";
            }

            if (actor is Player)
            {
                return "player";
            }

            return actor is Game ? "game" : "gizmo";
        }

        /// <summary>"Player 1" for the first slot.</summary>
        private static string PlayerName(Player player)
        {
            return "Player " + (player.Slot + 1);
        }

        /// <summary>A number rounded to the nearest whole one, halves away from 0.</summary>
        private static int Round(float value)
        {
            return value < 0.0f ? -(int)(-value + 0.5f) : (int)(value + 0.5f);
        }
    }
}
#endif
