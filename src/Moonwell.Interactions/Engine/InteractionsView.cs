#if DEBUG
using System.Collections.Generic;

namespace Moonwell.Engine
{
    /// <summary>
    /// The interactions package's view in the debug mode: the gizmos, and the items and destructables of kinds already
    /// tracked, near a click; a selected object's cell, interactions and pairs, in the panel and drawn; and the cycle's cost
    /// and counts. It reads the grid and the entries as they are and never asks for a kind (selecting starts no tracking);
    /// the cycle counts into plain int fields, so a step makes no table.
    /// </summary>
    internal sealed class InteractionsView : IDebugView
    {
        /// <summary>The steps the statistics keep: a second at the default step.</summary>
        private const int Steps = 32;

        /// <summary>The most pair lines drawn: the nearest.</summary>
        private const int MostPairLines = 32;

        /// <summary>A step counts in the last second when it ran less than this long ago in Moonwell's time; a thousandth is forgiven, as a step a whole second back can come out a little under in a float.</summary>
        private const float Second = 0.999f;

        /// <summary>The view; made on first use.</summary>
        private static InteractionsView? instance;

        /// <summary>The real seconds each of the last 32 steps took, at its place; 0 for a step taken while the mode was off.</summary>
        private static float[] spent = new float[Steps];

        /// <summary>Moonwell's time each of the last 32 steps ran at, at its place.</summary>
        private static float[] times = new float[Steps];

        /// <summary>Whether a step has run at each place.</summary>
        private static bool[] ran = new bool[Steps];

        /// <summary>The place the next step closes.</summary>
        private static int slot;

        /// <summary>The counts of each class of interaction, in the order a class was first started.</summary>
        private static List<InteractionStats> classes = new List<InteractionStats>();

        /// <summary>The entries a click's cells hold; emptied after each use.</summary>
        private readonly List<Entry> found = new List<Entry>();

        /// <summary>The pairs being shown, nearest first.</summary>
        private readonly List<Owner> pairs = new List<Owner>();

        /// <summary>The interaction of each pair in pairs.</summary>
        private readonly List<Interaction> pairsOf = new List<Interaction>();

        /// <summary>The distance of each pair in pairs.</summary>
        private readonly List<float> distances = new List<float>();

        /// <summary>The facts of the pair read last.</summary>
        private readonly PairFacts facts = new PairFacts();

        /// <summary>The view, made on first use; Interactions.Install adds it to the debug mode.</summary>
        internal static InteractionsView Instance
        {
            get
            {
                if (instance == null)
                {
                    instance = new InteractionsView();
                }

                return instance;
            }
        }

        /// <summary>Forgets the statistics and the classes counted; Interactions.Install calls it, as an install starts the package afresh. An interaction of an earlier install keeps its old counts, which no list holds.</summary>
        internal static void Reset()
        {
            spent = new float[Steps];
            times = new float[Steps];
            ran = new bool[Steps];
            slot = 0;
            classes = new List<InteractionStats>();
        }

        /// <summary>An interaction has started with its holder in the grid: its class's count goes up, and it is given the counts to keep.</summary>
        internal static InteractionStats Started(Interaction interaction)
        {
            var kind = Names.ClassOf(interaction);
            InteractionStats? stats = null;
            for (var i = 0; i < classes.Count; i++)
            {
                if (classes[i].Class == kind)
                {
                    stats = classes[i];
                    break;
                }
            }

            if (stats == null)
            {
                stats = new InteractionStats(kind, Names.NameOf(interaction), Steps);
                classes.Add(stats);
            }

            stats.Count++;
            return stats;
        }

        /// <summary>A step of the cycle has run: its real seconds, Moonwell's time and each class's ticks are kept at its place, and the oldest of the 32 goes.</summary>
        internal static void StepRan(float seconds)
        {
            spent[slot] = seconds;
            times[slot] = Runtime.World.Now();
            ran[slot] = true;
            for (var i = 0; i < classes.Count; i++)
            {
                var stats = classes[i];
                stats.Ticks[slot] = stats.StepTicks;
                stats.StepTicks = 0;
            }

            slot = (slot + 1) % Steps;
        }

        /// <summary>Adds the gizmos, items and destructables in the grid's cells within the radius of a point; units are the session's. Only the kinds tracked are in the grid, and the grid is read as it is, so no kind starts being tracked.</summary>
        public void Candidates(float x, float y, float radius, List<Actor> into)
        {
            var grid = Interactions.GridIfMade;
            if (grid == null)
            {
                return;
            }

            Raw.Truncate(found, 0);
            grid.Collect(x - radius, y - radius, x + radius, y + radius, found);
            var count = found.Count;
            for (var i = 0; i < count; i++)
            {
                var entry = Raw.At(found, i);
                if (entry.Kind == Tracking.UnitKind || entry.Ended || !entry.HasPlace || !Tracking.Asked(entry.Kind))
                {
                    continue;
                }

                var dx = entry.X - x;
                var dy = entry.Y - y;
                if (dx * dx + dy * dy <= radius * radius && !into.Contains(entry.Thing))
                {
                    into.Add(entry.Thing);
                }
            }

            Raw.Truncate(found, 0);
        }

        /// <summary>The place of a gizmo, or of any thing tracked, as its entry has it; false for one not tracked, ended or with no place.</summary>
        public bool Place(Actor actor, out float x, out float y)
        {
            x = 0.0f;
            y = 0.0f;
            var entry = Interactions.IsInstalled ? Tracking.EntryOf(actor) : null;
            if (entry == null || !entry.HasPlace)
            {
                return false;
            }

            x = entry.X;
            y = entry.Y;
            return true;
        }

        /// <summary>
        /// "cell 8, 8"; for each interaction it holds "FireAura: units, range 300, tick 0.5 s, 2 pairs" and its pairs nearest
        /// first, "  Footman 2   100 · met 0.5 s ago · ticked 2", with " · waits 0.3 s", " · cooldown 0.5 s" and " · paused" as
        /// they apply; then for each pair it is the other of, "paired by: FireAura on Footman 1 · 100 · met 0.5 s ago". Nothing
        /// for a thing not tracked.
        /// </summary>
        public void Lines(Actor selected, DebugLines lines)
        {
            var entry = Interactions.IsInstalled ? Tracking.EntryOf(selected) : null;
            if (entry == null)
            {
                return;
            }

            var grid = Interactions.grid;
            if (grid != null && entry.Cell >= 0)
            {
                lines.Add("cell " + grid.ColumnOfCell(entry.Cell) + ", " + grid.RowOfCell(entry.Cell));
            }

            var held = entry.Held;
            if (held != null)
            {
                for (var i = 0; i < held.Count; i++)
                {
                    var interaction = held[i];
                    lines.Add(InteractionLine(interaction));
                    ClearPairs();
                    AddPairs(interaction);
                    for (var p = 0; p < pairs.Count; p++)
                    {
                        interaction.ReadPair(pairs[p], facts);
                        lines.Add(PairLine(interaction));
                    }
                }
            }

            var partners = entry.Partners;
            if (partners != null)
            {
                for (var i = 0; i < partners.Count; i++)
                {
                    var pair = partners[i];
                    var of = pair.Owner as Interaction;
                    if (of == null)
                    {
                        continue;
                    }

                    of.ReadPair(pair, facts);
                    lines.Add("paired by: " + Names.NameOf(of) + " on " + DebugPanel.NameOf(facts.Holder) + " · " + DebugPanel.Whole(facts.Distance) + " · met " + DebugPanel.Tenths(facts.Since) + " s ago");
                }
            }

            ClearPairs();
        }

        /// <summary>
        /// A circle of each Range held; the grid lines across the cells the checks look at (Range + Slack of the largest
        /// range, from the place the grid has), or the object's own cell when it holds none; a line to each pair's other, the
        /// 32 nearest, Paused, Waiting (a wait or cooldown running) or Ticking. Core draws the marker.
        /// </summary>
        public void Draw(Actor selected, DebugDraw draw)
        {
            var entry = Interactions.IsInstalled ? Tracking.EntryOf(selected) : null;
            if (entry == null)
            {
                return;
            }

            // Its own statement: an out variable declared in a condition reaches no further in Lua after ||.
            var placed = DebugSession.PlaceOf(selected, out var x, out var y);
            if (!placed)
            {
                return;
            }

            var largest = 0.0f;
            var held = entry.Held;
            if (held != null)
            {
                for (var i = 0; i < held.Count; i++)
                {
                    var range = held[i].range;
                    draw.Circle(DebugDraw.Neutral, x, y, range);
                    if (range > largest)
                    {
                        largest = range;
                    }
                }
            }

            var grid = Interactions.grid;
            if (grid != null && entry.Cell >= 0)
            {
                if (largest > 0.0f)
                {
                    var reach = largest + Interactions.Current().Slack;
                    GridLines(draw, grid, grid.ColumnAt(entry.X - reach), grid.ColumnAt(entry.X + reach), grid.RowAt(entry.Y - reach), grid.RowAt(entry.Y + reach));
                }
                else
                {
                    var column = grid.ColumnOfCell(entry.Cell);
                    var row = grid.RowOfCell(entry.Cell);
                    GridLines(draw, grid, column, column, row, row);
                }
            }

            ClearPairs();
            if (held != null)
            {
                for (var i = 0; i < held.Count; i++)
                {
                    AddPairs(held[i]);
                }
            }

            var partners = entry.Partners;
            if (partners != null)
            {
                for (var i = 0; i < partners.Count; i++)
                {
                    var of = partners[i].Owner as Interaction;
                    if (of != null)
                    {
                        AddPair(of, partners[i]);
                    }
                }
            }

            var lines = pairs.Count < MostPairLines ? pairs.Count : MostPairLines;
            for (var i = 0; i < lines; i++)
            {
                var of = pairsOf[i];
                of.ReadPair(pairs[i], facts);
                var holding = facts.Holder == selected;
                var end = holding ? facts.Other : facts.Holder;
                var endPlaced = DebugSession.PlaceOf(end, out var endX, out var endY);
                if (!endPlaced)
                {
                    endX = holding ? facts.OtherX : facts.HolderX;
                    endY = holding ? facts.OtherY : facts.HolderY;
                }

                draw.Line(StyleOf(of), x, y, endX, endY);
            }

            ClearPairs();
        }

        /// <summary>
        /// "cycle 2.4 ms a step (worst 4) · 32 steps a second" over the last 32 steps; "tracked: 200 units, 0 items, 0
        /// destructables, 50 gizmos · 31 cells"; "interactions 100 · pairs 412 · ticks 3,100 a second"; then a line per class,
        /// "FireAura  50 · 380 pairs · 3,040 ticks a second", most pairs first, in the order first started when equal.
        /// </summary>
        public void Stats(DebugLines lines)
        {
            var now = Runtime.World.Now();
            var steps = 0;
            var recent = 0;
            var sum = 0.0f;
            var worst = 0.0f;
            for (var i = 0; i < Steps; i++)
            {
                if (!ran[i])
                {
                    continue;
                }

                steps++;
                sum += spent[i];
                if (spent[i] > worst)
                {
                    worst = spent[i];
                }

                if (now - times[i] < Second)
                {
                    recent++;
                }
            }

            var average = steps > 0 ? sum / steps : 0.0f;
            lines.Add("cycle " + DebugPanel.Tenths(average * 1000.0f) + " ms a step (worst " + DebugPanel.Whole(worst * 1000.0f) + ") · " + Plural(recent, "step") + " a second");
            var grid = Interactions.grid;
            var cells = grid != null ? grid.Made : 0;
            lines.Add("tracked: " + Plural(Tracking.CountOf(Tracking.UnitKind), "unit") + ", " + Plural(Tracking.CountOf(Tracking.ItemKind), "item") + ", " + Plural(Tracking.CountOf(Tracking.DestructableKind), "destructable") + ", " + Plural(Tracking.CountOf(Tracking.GizmoKind), "gizmo") + " · " + Plural(cells, "cell"));

            // The classes most pairs first, in the order first started when equal; and the totals.
            var order = new List<InteractionStats>();
            var ticks = new List<int>();
            var interactions = 0;
            var pairTotal = 0;
            var tickTotal = 0;
            for (var i = 0; i < classes.Count; i++)
            {
                var stats = classes[i];
                var classTicks = 0;
                for (var s = 0; s < Steps; s++)
                {
                    if (ran[s] && now - times[s] < Second)
                    {
                        classTicks += stats.Ticks[s];
                    }
                }

                interactions += stats.Count;
                pairTotal += stats.Pairs;
                tickTotal += classTicks;
                if (stats.Count == 0 && classTicks == 0)
                {
                    continue;
                }

                var at = order.Count;
                while (at > 0 && order[at - 1].Pairs < stats.Pairs)
                {
                    at--;
                }

                order.Insert(at, stats);
                ticks.Insert(at, classTicks);
            }

            lines.Add("interactions " + DebugPanel.Count(interactions) + " · pairs " + DebugPanel.Count(pairTotal) + " · ticks " + DebugPanel.Count(tickTotal) + " a second");
            for (var i = 0; i < order.Count; i++)
            {
                var stats = order[i];
                lines.Add(stats.Name + "  " + DebugPanel.Count(stats.Count) + " · " + Plural(stats.Pairs, "pair") + " · " + Plural(ticks[i], "tick") + " a second");
            }
        }

        /// <summary>"FireAura: units, range 300, tick 0.5 s, 2 pairs": the other kind, the range, the tick as the cycle runs it, and the pairs.</summary>
        private static string InteractionLine(Interaction interaction)
        {
            var tick = Cycle.Steps(interaction.tick) * Interactions.Current().Step;
            return Names.NameOf(interaction) + ": " + OtherName(interaction) + ", range " + DebugPanel.Whole(interaction.range) + ", tick " + Seconds(tick) + " s, " + Plural(interaction.pairs.Count, "pair");
        }

        /// <summary>"  Footman 2   100 · met 0.5 s ago · ticked 2 · cooldown 0.5 s": the pair in facts, of an interaction.</summary>
        private string PairLine(Interaction interaction)
        {
            var line = "  " + DebugPanel.NameOf(facts.Other) + "   " + DebugPanel.Whole(facts.Distance) + " · met " + DebugPanel.Tenths(facts.Since) + " s ago · ticked " + DebugPanel.Count(facts.Ticked);
            if (facts.Waits > 0.0f)
            {
                line = line + " · waits " + DebugPanel.Tenths(facts.Waits) + " s";
            }

            if (facts.Cooldown > 0.0f)
            {
                line = line + " · cooldown " + DebugPanel.Tenths(facts.Cooldown) + " s";
            }

            if (facts.Paused || interaction.paused)
            {
                line = line + " · paused";
            }

            return line;
        }

        /// <summary>The style of the line of the pair in facts: Paused when it or its interaction is paused, Waiting while a wait or cooldown runs, else Ticking.</summary>
        private int StyleOf(Interaction interaction)
        {
            if (facts.Paused || interaction.paused)
            {
                return DebugDraw.Paused;
            }

            return facts.Waits > 0.0f || facts.Cooldown > 0.0f ? DebugDraw.Waiting : DebugDraw.Ticking;
        }

        /// <summary>The other class as the panel names it: the kind of thing in the plural when it is exactly one, else the class's name.</summary>
        private static string OtherName(Interaction interaction)
        {
            var kind = interaction.otherKind;
            if (kind == Tracking.UnitKind)
            {
                return "units";
            }

            if (kind == Tracking.ItemKind)
            {
                return "items";
            }

            if (kind == Tracking.DestructableKind)
            {
                return "destructables";
            }

            return kind == Tracking.GizmoKind ? "gizmos" : interaction.OtherClassName();
        }

        /// <summary>The grid lines across a block of cells: an upright line at each column's edge, left to right, then a level line at each row's edge, bottom to top.</summary>
        private static void GridLines(DebugDraw draw, Grid grid, int firstColumn, int lastColumn, int firstRow, int lastRow)
        {
            var left = grid.ColumnLeft(firstColumn);
            var right = grid.ColumnLeft(lastColumn + 1);
            var bottom = grid.RowBottom(firstRow);
            var top = grid.RowBottom(lastRow + 1);
            for (var column = firstColumn; column <= lastColumn + 1; column++)
            {
                var x = grid.ColumnLeft(column);
                draw.Line(DebugDraw.Neutral, x, bottom, x, top);
            }

            for (var row = firstRow; row <= lastRow + 1; row++)
            {
                var y = grid.RowBottom(row);
                draw.Line(DebugDraw.Neutral, left, y, right, y);
            }
        }

        /// <summary>Empties the lists of pairs being shown.</summary>
        private void ClearPairs()
        {
            pairs.Clear();
            pairsOf.Clear();
            distances.Clear();
        }

        /// <summary>Adds an interaction's pairs to the pairs being shown, each at its place by distance.</summary>
        private void AddPairs(Interaction interaction)
        {
            var list = interaction.pairs;
            for (var i = 0; i < list.Count; i++)
            {
                AddPair(interaction, list[i]);
            }
        }

        /// <summary>Adds a pair of an interaction to the pairs being shown, after those nearer or as near.</summary>
        private void AddPair(Interaction interaction, Owner pair)
        {
            interaction.ReadPair(pair, facts);
            var distance = facts.Distance;
            var at = pairs.Count;
            while (at > 0 && distances[at - 1] > distance)
            {
                at--;
            }

            pairs.Insert(at, pair);
            pairsOf.Insert(at, interaction);
            distances.Insert(at, distance);
        }

        /// <summary>"1 pair", "2 pairs": a count and a noun, plural unless the count is 1.</summary>
        private static string Plural(int count, string noun)
        {
            return DebugPanel.Count(count) + " " + noun + (count == 1 ? "" : "s");
        }

        /// <summary>Seconds to the hundredth, with no zeros at the end: "0.5", "0.03", "2".</summary>
        private static string Seconds(float seconds)
        {
            var hundredths = (int)(seconds * 100.0f + 0.5f);
            var whole = DebugPanel.Count(hundredths / 100);
            var rest = hundredths % 100;
            if (rest == 0)
            {
                return whole;
            }

            if (rest % 10 == 0)
            {
                return whole + "." + rest / 10;
            }

            return whole + "." + (rest < 10 ? "0" : "") + rest;
        }
    }
}
#endif
