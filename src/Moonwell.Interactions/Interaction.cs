using System;
using System.Collections.Generic;
using Moonwell.Engine;

namespace Moonwell
{
    /// <summary>
    /// What one thing, the holder, does with the things of a class near it. A map derives from Interaction&lt;THolder, TOther&gt;;
    /// this base carries what the cycle and Interact need without the types. An interaction belongs to the owner that started
    /// it and ends with it, with its holder, or by End; its pairs part first, newest first.
    /// </summary>
    public abstract class Interaction : Scope
    {
        // The fields are internal, not private, as a map's subclass may name a field of its own the same:
        // CSharp.lua gives a subclass's field a name of its own only when it sees the base's field of that name,
        // and it does not see a private one, so in Lua the two would be one field.

        /// <summary>The pairs, in the order they met; each is an Interaction&lt;THolder, TOther&gt;.Pair, kept as an Scope (see Pairing).</summary>
        internal readonly List<Scope> pairs = new List<Scope>();

        /// <summary>The pair of each other, by the other's entry id; a lookup only.</summary>
        internal readonly Dictionary<int, Scope> byOther = new Dictionary<int, Scope>();

        /// <summary>The read-only view of the pairs that Pairs gives; made with the interaction.</summary>
        internal object? view;

        /// <summary>The holder's entry; null before the interaction starts.</summary>
        internal Entry? holderEntry;

        /// <summary>How many checks have run; a pair or a stopped other kept at a check is marked with it.</summary>
        internal int checks;

        /// <summary>The entries of the others whose pair was stopped and that have not been out of range since, in the order they were stopped; null until a pair is stopped.</summary>
        internal List<Entry>? stopped;

        /// <summary>The check at which each stopped other was last kept, by its entry id; a lookup only.</summary>
        internal Dictionary<int, int>? stoppedSeen;

        /// <summary>The entry ids of the others whose pair Accepts refused at the check refusedCheck, so that check does not ask them again; a lookup only, null until a pair is refused.</summary>
        internal Dictionary<int, bool>? refused;

        /// <summary>The check refused holds the refusals of; 0 before any.</summary>
        internal int refusedCheck;

        /// <summary>The interaction's place in the cycle; made when it starts.</summary>
        internal InteractionDue? due;

        /// <summary>Whether Interact has started it.</summary>
        internal bool started;

        /// <summary>Whether it is paused.</summary>
        internal bool paused;

        /// <summary>The range.</summary>
        internal float range;

        /// <summary>The seconds between checks.</summary>
        internal float tick;

        /// <summary>Whether a check asks Accepts again for the pairs that have met; true unless the map turns it off.</summary>
        internal bool recheckAccepts;

        /// <summary>The kind of thing TOther is exactly (Tracking.ExactKind), which a check compares an entry's kind with; 0 when it tests the thing's class.</summary>
        internal int otherKind;
#if DEBUG

        /// <summary>The debug mode's counts of the interaction's class; null until it has started with a holder in the grid.</summary>
        internal InteractionStats? debugStats;
#endif

        /// <summary>Only Interaction&lt;THolder, TOther&gt; derives from it.</summary>
        internal Interaction()
        {
            // In Lua a field left at its default is the class's, read up the class chain; the check reads these at every
            // reaction, so the interaction keeps them as its own. They are false already.
            ending = false;
            paused = false;
            recheckAccepts = true;
            refusedCheck = 0;
        }

        /// <summary>How near, centre to centre and flat, the other must be to the holder to meet it, in game units; required, more than 0. A map sets it in its constructor; a change takes effect at the next check.</summary>
        [LocalSafe]
        public float Range
        {
            get { return range; }
            protected set { range = value; }
        }

        /// <summary>The seconds between checks, rounded up to whole steps of the cycle and at least one step; a step unless set. The first check comes within a tick of Interact: interactions of the same tick started in the same step take turns over its steps in the order they start, so many started at once do not all check at the same step. A map sets it in its constructor; a change takes effect after the check that is waiting.</summary>
        [LocalSafe]
        public float Tick
        {
            get { return tick; }
            protected set
            {
                Dev.Check(value >= 0.0f, Names.NameOf(this) + ".Tick: seconds must be 0 or more");
                tick = value;
            }
        }

        /// <summary>Whether Accepts is asked again, before each tick, for the pairs that have met: a no parts the pair (OnPart), and the other may meet again at a later tick once Accepts says yes. True unless set; false asks only before a meeting and saves a call per pair at each tick. A map sets it in its constructor; a change takes effect at the next check.</summary>
        [LocalSafe]
        public bool RecheckAccepts
        {
            get { return recheckAccepts; }
            protected set { recheckAccepts = value; }
        }

        /// <summary>Whether the interaction is paused: it makes no checks, and its pairs are kept.</summary>
        [LocalSafe]
        public bool IsPaused
        {
            get { return paused; }
        }

        /// <summary>Whether the interaction should stop the check it is in: it is paused, or it or its holder has begun to end. An object that has ended began to end first, so ending covers HasEnded.</summary>
        internal bool Over
        {
            get { return paused || ending || (holderEntry != null && holderEntry.Ended); }
        }

        /// <summary>Stops the checks, the one running too when a reaction pauses it: no pair meets, ticks or parts until Resume, and the pairs are kept.</summary>
        public void Pause()
        {
            paused = true;
        }

        /// <summary>Starts the checks again, the next a tick from now unless one is waiting.</summary>
        public void Resume()
        {
            paused = false;
            if (due != null)
            {
                due.Arm();
            }
        }

        /// <summary>Whether a pair or another owned object has begun to end. An object that has ended began to end first, so ending covers HasEnded.</summary>
        internal static bool Gone(Scoped owned)
        {
            return owned.ending;
        }

        /// <summary>Whether a thing is of the holder class.</summary>
        internal abstract bool HeldBy(Actor holder);

        /// <summary>Keeps the holder, typed.</summary>
        internal abstract void BindHolder(Actor holder);

        /// <summary>Asks for the kinds of thing the other class covers to be tracked.</summary>
        internal abstract void NeedOthers();

        /// <summary>The check at a tick: what is near now meets, ticks or parts. It gives the error the package's own code raised in it, or null; the reactions' errors are reported where they run.</summary>
        internal abstract Exception? RunCheck();
#if DEBUG

        /// <summary>The name of the other class, as the debug mode shows it when it is not one kind of thing.</summary>
        internal abstract string OtherClassName();

        /// <summary>Reads a pair of the interaction into the debug mode's facts; only the typed class can cast it.</summary>
        internal abstract void ReadPair(Scope pair, PairFacts facts);
#endif

        /// <summary>The check itself, from the holder's read on; RunCheck runs it guarded, and takes what it added off the shared lists after, as a finally would.</summary>
        internal abstract void CheckNear();

        /// <summary>Runs OnPart for a pair that is parting, guarded.</summary>
        internal abstract void RunPart(Scope pair);

        /// <summary>Starts the interaction on a holder of a kind, for an owner or else the current one, with its first check within a tick: Interact's work. Fails in a development build when it was started before, when the holder is not of the holder class, or when Range is not more than 0. The holder is tracked on its own, without asking for its whole kind (Tracking.TrackHolder); the kinds of the others are asked for. A holder that has ended, or an owner that has, ends it at once.</summary>
        internal void Begin(Actor holder, int kind, Scope? scope)
        {
            Dev.Check(!started, Names.NameOf(this) + ".Interact: the interaction has been started already");
            Dev.Check(HeldBy(holder), Names.NameOf(this) + ".Interact: a " + Names.NameOf(holder) + " cannot hold it");
            Dev.Check(range > 0.0f, Names.NameOf(this) + ".Interact: Range must be more than 0");
            started = true;
            Interactions.EnsureInstalled();
            NeedOthers();
            BindHolder(holder);
            (scope ?? Scope.Current).Adopt(this);
            if (HasEnded)
            {
                return;
            }

            var entry = Tracking.TrackHolder(holder, kind);
            if (entry == null)
            {
                End();
                return;
            }

            holderEntry = entry;
            Tracking.Hold(entry, this);
            due = new InteractionDue(this);
            due.ArmFirst();
#if DEBUG
            debugStats = InteractionsView.Started(this);
#endif
        }

        /// <summary>Joins a new pair to the interaction: it is the interaction's, last in meeting order, and found by its other's entry.</summary>
        internal void Join(Scope pair, Entry other)
        {
            Adopt(pair);
            pairs.Add(pair);
            byOther[other.Id] = pair;
            Tracking.AddPartner(other, pair);
#if DEBUG
            if (debugStats != null)
            {
                debugStats.Pairs++;
            }
#endif
        }

        /// <summary>Takes a parting pair out of the interaction's lists and its other's; one already out is passed over.</summary>
        internal void Unpair(Scope pair, Entry other)
        {
            for (var i = pairs.Count - 1; i >= 0; i--)
            {
                if (Raw.At(pairs, i) == pair)
                {
                    pairs.RemoveAt(i);
#if DEBUG
                    if (debugStats != null)
                    {
                        debugStats.Pairs--;
                    }
#endif
                    break;
                }
            }

            Scope? found;
            if (byOther.TryGetValue(other.Id, out found) && found == pair)
            {
                byOther.Remove(other.Id);
            }

            Tracking.RemovePartner(other, pair);
        }

        /// <summary>Adds the pairs, in meeting order, to the end of the shared walk; returns where they start.</summary>
        internal int WalkPairs()
        {
            var walk = Pairing.walk;
            var start = walk.Count;
            var count = pairs.Count;
            for (var i = 0; i < count; i++)
            {
                Raw.Append(walk, Raw.At(pairs, i));
            }

            return start;
        }

        /// <summary>Parts every pair, in meeting order, as at a check of a holder with no place; stops when a reaction pauses or ends the interaction or ends its holder.</summary>
        internal void PartAll()
        {
            // It runs inside the check's guard, which takes the walk back to where it was however the check ends, so a
            // finally here, two closures in Lua, is not needed.
            var walk = Pairing.walk;
            var start = WalkPairs();
            var end = walk.Count;
            for (var i = start; i < end; i++)
            {
                var pair = Raw.At(walk, i);
                if (Gone(pair))
                {
                    continue;
                }

                pair.End();
                if (Over)
                {
                    Pairing.Release(start);
                    return;
                }
            }

            Pairing.Release(start);
        }

        /// <summary>Notes that the pair with an other was stopped: the two do not meet again until the other has been out of range at a check.</summary>
        internal void Stopping(Entry other)
        {
            if (stopped == null || stoppedSeen == null)
            {
                stopped = new List<Entry>();
                stoppedSeen = new Dictionary<int, int>();
            }

            if (!stoppedSeen.ContainsKey(other.Id))
            {
                stopped.Add(other);
            }

            stoppedSeen[other.Id] = checks;
        }

        /// <summary>Whether an other's pair was stopped and it has not been out of range since.</summary>
        internal bool IsStopped(Entry other)
        {
            return stoppedSeen != null && stoppedSeen.ContainsKey(other.Id);
        }

        /// <summary>Marks a stopped other as kept at this check; an other that was not stopped is passed over.</summary>
        internal void SeeStopped(Entry other)
        {
            if (stoppedSeen != null && stoppedSeen.ContainsKey(other.Id))
            {
                stoppedSeen[other.Id] = checks;
            }
        }

        /// <summary>Notes that Accepts refused an other's pair at this check, so the check does not ask it again as a new other.</summary>
        internal void Refuse(Entry other)
        {
            if (refused == null)
            {
                refused = new Dictionary<int, bool>();
            }

            if (refusedCheck != checks)
            {
                refused.Clear();
                refusedCheck = checks;
            }

            refused[other.Id] = true;
        }

        /// <summary>Forgets the stopped others that were not kept at this check, so they meet again when next in range. One the check did not collect is read directly first, as for a pair (StillNear): within range of the holder's place it is kept.</summary>
        internal void ForgetStopped(float hx, float hy, float limit)
        {
            if (stopped == null || stoppedSeen == null)
            {
                return;
            }

            var kept = 0;
            var count = stopped.Count;
            for (var i = 0; i < count; i++)
            {
                var other = Raw.At(stopped, i);
                if (stoppedSeen[other.Id] == checks || StillNear(other, hx, hy, limit))
                {
                    Raw.Put(stopped, kept, other);
                    kept++;
                }
                else
                {
                    stoppedSeen.Remove(other.Id);
                }
            }

            Raw.Truncate(stopped, kept);
        }

        /// <summary>Forgets every stopped other, as at a check of a holder with no place, when every other is out of reach.</summary>
        internal void ForgetAllStopped()
        {
            if (stopped == null || stoppedSeen == null)
            {
                return;
            }

            stopped.Clear();
            stoppedSeen.Clear();
        }

        /// <summary>Whether an other that a check did not collect is within range of the holder's place all the same, given the squared range: it is read directly, at most once a step as every read is. A unit or item moved further than Slack in one jump (a teleport, both moved by code) is still in the cell it was last read in until its cell check, so the check's box can miss it while it stands next to the holder.</summary>
        internal static bool StillNear(Entry other, float hx, float hy, float limit)
        {
            Tracking.ReadNow(other);
            if (!other.HasPlace)
            {
                return false;
            }

            var dx = other.X - hx;
            var dy = other.Y - hy;
            return dx * dx + dy * dy <= limit;
        }

        /// <summary>Ends the checks, forgets the holder, and parts the pairs, newest first; the interaction's other children end after.</summary>
        protected sealed override void OnEnd()
        {
            if (due != null)
            {
                due.Cancel();
            }

            if (holderEntry != null)
            {
                Tracking.Unhold(holderEntry, this);
            }
#if DEBUG

            // The pairs are taken out here, before they end, so Unpair does not count them out again.
            if (debugStats != null)
            {
                debugStats.Count--;
                debugStats.Pairs -= pairs.Count;
            }
#endif

            while (pairs.Count > 0)
            {
                var pair = pairs[pairs.Count - 1];
                pairs.RemoveAt(pairs.Count - 1);
                pair.End();
            }
        }
    }

    /// <summary>
    /// What a holder of class THolder does with the things of class TOther near it: a map's aura, link or missile hit. The
    /// constructor sets Range, and Tick unless a check at every step is wanted. At each tick the interaction checks what is
    /// within Range: a new other is asked about with Accepts and, if accepted, meets (OnMeet, then OnTick); an other still
    /// in range is asked again, unless RecheckAccepts is off, and ticks (OnTick); one refused, gone out of range or with
    /// no place parts (OnPart). Reactions run guarded, the pair's with the pair as the current owner and Accepts with the
    /// interaction.
    /// </summary>
    /// <typeparam name="THolder">The class of the holder: Unit, Item, Destructable, Gizmo, a map's class of one of them, or Actor.</typeparam>
    /// <typeparam name="TOther">The class of the things it pairs with, as Find takes: Unit, Item, Destructable, Gizmo, a map's gizmo class, or Actor.</typeparam>
    public abstract class Interaction<THolder, TOther> : Interaction
        where THolder : Actor
        where TOther : Actor
    {
        // Internal, not private, for the same reason as Interaction's fields.

        /// <summary>The holder; null before the interaction starts.</summary>
        internal THolder holder = null!;

        /// <summary>Makes the interaction with its view of the pairs, so a read of Pairs makes nothing.</summary>
        protected Interaction()
        {
            Pairing.MakeView<THolder, TOther>(this);
            otherKind = Tracking.ExactKind(typeof(TOther));
        }

        /// <summary>The thing that holds the interaction.</summary>
        [LocalSafe]
        public THolder Holder
        {
            get { return holder; }
        }

        /// <summary>The pairs, in the order they met: a view that follows them as they meet and part, the same one at every read.</summary>
        [LocalSafe]
        public IReadOnlyList<Pair> Pairs
        {
            get { return Pairing.View<THolder, TOther>(this); }
        }

        /// <summary>Whether an other within range may meet the holder, and, asked again before each tick while RecheckAccepts is on (the default), whether a pair that has met goes on; true unless a map says. Refused, a new other does not meet and is asked again at the next tick; a pair parts (OnPart), and its other is asked again from the next tick and meets again once accepted. An error it throws is reported and refuses. Runs with the interaction as the current owner.</summary>
        public virtual bool Accepts(TOther other)
        {
            return true;
        }

        /// <summary>Runs when the two meet, with the pair as the current owner; OnTick runs right after, in the same check.</summary>
        public virtual void OnMeet(Pair pair)
        {
        }

        /// <summary>Runs at each tick while the two are in range, unless the pair waits or is paused, with the pair as the current owner.</summary>
        public virtual void OnTick(Pair pair)
        {
        }

        /// <summary>Runs once when the pair ends, for any reason: out of range, a side with no place or ending, Accepts refusing it, Stop, or the interaction ending. The pair is the current owner; what it made ends after.</summary>
        public virtual void OnPart(Pair pair)
        {
        }

        /// <summary>Whether a thing is a THolder.</summary>
        internal sealed override bool HeldBy(Actor thing)
        {
            return thing is THolder;
        }

        /// <summary>Keeps the holder as a THolder.</summary>
        internal sealed override void BindHolder(Actor thing)
        {
            holder = (THolder)thing;
        }

        /// <summary>Asks for the kinds TOther covers.</summary>
        internal sealed override void NeedOthers()
        {
            Tracking.NeedFor(typeof(TOther));
        }

        /// <summary>Runs OnPart for a parting pair.</summary>
        internal sealed override void RunPart(Scope pair)
        {
            CallPart(Pairing.As<THolder, TOther>(pair));
        }
#if DEBUG

        /// <summary>The name of TOther.</summary>
        internal sealed override string OtherClassName()
        {
            return typeof(TOther).Name;
        }

        /// <summary>Reads a pair, as the Pair it is, into the debug mode's facts.</summary>
        internal sealed override void ReadPair(Scope owner, PairFacts facts)
        {
            Pair pair = Pairing.As<THolder, TOther>(owner);
            facts.Holder = pair.holder;
            facts.Other = pair.other;
            facts.Distance = pair.Distance;
            facts.HolderX = pair.HolderX;
            facts.HolderY = pair.HolderY;
            facts.OtherX = pair.OtherX;
            facts.OtherY = pair.OtherY;
            facts.Since = pair.Since;
            facts.Waits = pair.Waits;
            facts.Cooldown = pair.Cooldown;
            facts.Paused = pair.IsPaused;
            facts.Ticked = pair.ticked;
        }
#endif

        /// <summary>
        /// The check at a tick, in this order. The holder is read; with no place every pair parts and the check ends. The
        /// things within Range plus Slack are collected, and those of class TOther, not the holder, with a place and within
        /// Range are kept. The pairs, in meeting order: one whose other is not kept parts, unless a direct read of the other
        /// finds it within Range (one moved far in one jump is still in the cell it was last read in); while RecheckAccepts
        /// is on, one still in range is asked Accepts again, paused or waiting too, and parts if refused. A stopped other
        /// that is not kept is forgotten, after the same direct read. The kept pairs tick, in meeting order, unless paused
        /// or waiting. The kept others with no pair, in the grid's order, are asked about unless stopped or refused in this
        /// check, and those accepted meet and tick. After any reaction that pauses or
        /// ends the interaction or ends its holder, the check stops. An error the package's own code raises stops the check
        /// and is given back, and the cycle's due reports it as ClassName.Check; a reaction's error is reported where it runs.
        /// </summary>
        internal sealed override Exception? RunCheck()
        {
            // A try and finally here would make two closures a check in Lua; Guard runs CheckNear with none. What it added
            // to the shared lists comes off after, as with the finally, and an error it raised is given to the cycle's due,
            // which reports it, so the cycle goes on.
            var start = Find.candidates.Count;
            var walkStart = Pairing.walk.Count;
            var failure = Guard.Check(this);
            Pairing.Release(walkStart);
            Find.Release(start);
            return failure;
        }

        /// <summary>The check itself, as RunCheck says: the holder's read, then the things near collected, kept, parted, ticked and met. It leaves on the shared lists what it added, for RunCheck to take off.</summary>
        internal sealed override void CheckNear()
        {
            var home = holderEntry!;
            checks++;
            Tracking.ReadNow(home);
            if (!home.HasPlace)
            {
                PartAll();
                ForgetAllStopped();
                return;
            }

            var hx = home.X;
            var hy = home.Y;

            // The check runs only once the package is installed, so it reads the settings and the grid as they are.
            var reach = range + Interactions.Current().Slack;
            var limit = range * range;
            var candidates = Find.candidates;
            var start = candidates.Count;
            var walk = Pairing.walk;
            var walkStart = walk.Count;
            var grid = Interactions.grid ?? Interactions.Grid;
            grid.Collect(hx - reach, hy - reach, hx + reach, hy + reach, candidates);
            var end = start;
            var count = candidates.Count;
            var kind = otherKind;
            for (var i = start; i < count; i++)
            {
                // An entry's kind says its thing's class: when TOther is exactly that class, the kinds compare for is.
                var entry = Raw.At(candidates, i);
                if (entry == home || (kind != 0 ? entry.Kind != kind : !(entry.Thing is TOther)))
                {
                    continue;
                }

                Tracking.ReadNow(entry);
                if (!entry.HasPlace)
                {
                    continue;
                }

                var dx = entry.X - hx;
                var dy = entry.Y - hy;
                if (dx * dx + dy * dy > limit)
                {
                    continue;
                }

                Raw.Put(candidates, end, entry);
                end++;
            }

            Find.Release(end);
            for (var i = start; i < end; i++)
            {
                var entry = Raw.At(candidates, i);
                Scope? found;
                if (byOther.TryGetValue(entry.Id, out found))
                {
                    Pair kept = Pairing.As<THolder, TOther>(found);
                    kept.Keep(checks, hx, hy, entry.X, entry.Y);
                }
                else
                {
                    SeeStopped(entry);
                }
            }

            var recheck = recheckAccepts;
            WalkPairs();
            var walkEnd = walk.Count;
            for (var i = walkStart; i < walkEnd; i++)
            {
                Pair pair = Pairing.As<THolder, TOther>(Raw.At(walk, i));
                if (Gone(pair))
                {
                    continue;
                }

                if (pair.kept != checks)
                {
                    // An other moved far in one jump may stand in range from a cell the box did not take in.
                    var other = pair.otherEntry;
                    if (!StillNear(other, hx, hy, limit))
                    {
                        pair.End();
                        if (Over)
                        {
                            return;
                        }

                        continue;
                    }

                    pair.Keep(checks, hx, hy, other.X, other.Y);
                }

                if (!recheck)
                {
                    continue;
                }

                // Asked again before each tick (decision 73): a no parts the pair, and the other is not asked again in
                // this check. Accepts may have ended the pair, its other or the interaction, or paused it.
                var accepted = CallAccepts(pair.other);
                if (Over)
                {
                    return;
                }

                if (accepted)
                {
                    continue;
                }

                Refuse(pair.otherEntry);
                if (!Gone(pair))
                {
                    pair.End();
                    if (Over)
                    {
                        return;
                    }
                }
            }

            ForgetStopped(hx, hy, limit);
            Pairing.Release(walkStart);
            WalkPairs();
            walkEnd = walk.Count;
            for (var i = walkStart; i < walkEnd; i++)
            {
                Pair pair = Pairing.As<THolder, TOther>(Raw.At(walk, i));
                if (Gone(pair) || !pair.Ready)
                {
                    continue;
                }

                CallTick(pair);
                if (Over)
                {
                    return;
                }
            }

            Pairing.Release(walkStart);
            var refusedNow = refusedCheck == checks ? refused : null;
            for (var i = start; i < end; i++)
            {
                var entry = Raw.At(candidates, i);
                if (entry.Ended || byOther.ContainsKey(entry.Id) || IsStopped(entry) || (refusedNow != null && refusedNow.ContainsKey(entry.Id)))
                {
                    continue;
                }

                var accepted = CallAccepts(Raw.As<TOther>(entry.Thing));
                if (Over)
                {
                    return;
                }

                if (!accepted || entry.Ended || byOther.ContainsKey(entry.Id))
                {
                    continue;
                }

                Pair pair = Pairing.Make<THolder, TOther>();
                pair.Begin(this, holder, Raw.As<TOther>(entry.Thing), entry, checks, hx, hy);
                Join(pair, entry);
                CallMeet(pair);
                if (Over)
                {
                    return;
                }

                if (Gone(pair) || !pair.Ready)
                {
                    continue;
                }

                CallTick(pair);
                if (Over)
                {
                    return;
                }
            }
        }

        /// <summary>Runs Accepts with the interaction as the current owner; an error it throws is reported and refuses.</summary>
        private bool CallAccepts(TOther other)
        {
            var previous = Scope.current;
            Scope.current = this;
            var failure = Guard.Accept(this, other);
            var accepted = Guard.answer;
            if (failure != null)
            {
                Errors.Report(Names.NameOf(this) + ".Accepts", failure);
            }

            Scope.current = previous;
            return accepted;
        }

        /// <summary>Runs OnMeet with the pair as the current owner, and reports an error it throws.</summary>
        private void CallMeet(Pair pair)
        {
            var previous = Scope.current;
            Scope.current = pair;
            var failure = Guard.Meet(this, pair);
            if (failure != null)
            {
                Errors.Report(Names.NameOf(this) + ".OnMeet", failure);
            }

            Scope.current = previous;
        }

        /// <summary>Runs OnTick with the pair as the current owner, and reports an error it throws.</summary>
        private void CallTick(Pair pair)
        {
#if DEBUG
            // The debug mode's counts: plain int fields, so a tick makes no table.
            pair.ticked++;
            if (debugStats != null)
            {
                debugStats.StepTicks++;
            }
#endif
            var previous = Scope.current;
            Scope.current = pair;
            var failure = Guard.Tick(this, pair);
            if (failure != null)
            {
                Errors.Report(Names.NameOf(this) + ".OnTick", failure);
            }

            Scope.current = previous;
        }

        /// <summary>Runs OnPart with the pair as the current owner, and reports an error it throws.</summary>
        private void CallPart(Pair pair)
        {
            var previous = Scope.current;
            Scope.current = pair;
            var failure = Guard.Part(this, pair);
            if (failure != null)
            {
                Errors.Report(Names.NameOf(this) + ".OnPart", failure);
            }

            Scope.current = previous;
        }

        /// <summary>
        /// A holder and an other that have met, from the meeting until they part. It belongs to its interaction, and what its
        /// reactions make (a timer) is the pair's and ends when it parts. Its reads come from the places read at the last
        /// check that kept it, as floats: in OnPart, for a pair that parts because the other went out of range or a side has
        /// no place, those are the check before. End parts the pair now, and an other still in range may meet again at once,
        /// in the same check; Stop parts it and keeps the two apart until the other has been out of range.
        /// </summary>
        public sealed class Pair : Scope
        {
            /// <summary>The interaction the pair is of.</summary>
            internal Interaction interaction = null!;

            /// <summary>The holder.</summary>
            internal THolder holder = null!;

            /// <summary>The other.</summary>
            internal TOther other = null!;

            /// <summary>The other's entry.</summary>
            internal Entry otherEntry = null!;

            /// <summary>The check that last kept the pair in range.</summary>
            internal int kept;

            /// <summary>The cycle's step the two met at.</summary>
            private int met;

            /// <summary>The first step the pair ticks at again after a Wait.</summary>
            private int waitUntil;

            /// <summary>The first step a cooldown has run out at.</summary>
            private int coolUntil;

            /// <summary>The holder's place at the last check, left to right.</summary>
            private float holderX;

            /// <summary>The holder's place at the last check, bottom to top.</summary>
            private float holderY;

            /// <summary>The other's place at the last check, left to right.</summary>
            private float otherX;

            /// <summary>The other's place at the last check, bottom to top.</summary>
            private float otherY;

            /// <summary>Whether the pair is paused.</summary>
            private bool paused;

            /// <summary>The map's own value.</summary>
            private object? data;
#if DEBUG

            /// <summary>How many times OnTick has run, for the debug mode.</summary>
            internal int ticked;
#endif

            /// <summary>Only Pairing makes a pair.</summary>
            internal Pair()
            {
                // As for the interaction: a check reads ending at every pair, and keeps it as the pair's own.
                ending = false;
            }

            /// <summary>The holder of the interaction.</summary>
            [LocalSafe]
            public THolder Holder
            {
                get { return holder; }
            }

            /// <summary>The thing the holder met.</summary>
            [LocalSafe]
            public TOther Other
            {
                get { return other; }
            }

            /// <summary>The distance between the two, centre to centre and flat, at the last check.</summary>
            [LocalSafe]
            public float Distance
            {
                get { return Maths.Distance(holderX, holderY, otherX, otherY); }
            }

            /// <summary>The angle from the holder to the other at the last check, in degrees, 0 to 360.</summary>
            [LocalSafe]
            public float Angle
            {
                get { return Maths.AngleBetween(holderX, holderY, otherX, otherY); }
            }

            /// <summary>The holder's place at the last check, left to right.</summary>
            [LocalSafe]
            public float HolderX
            {
                get { return holderX; }
            }

            /// <summary>The holder's place at the last check, bottom to top.</summary>
            [LocalSafe]
            public float HolderY
            {
                get { return holderY; }
            }

            /// <summary>The other's place at the last check, left to right.</summary>
            [LocalSafe]
            public float OtherX
            {
                get { return otherX; }
            }

            /// <summary>The other's place at the last check, bottom to top.</summary>
            [LocalSafe]
            public float OtherY
            {
                get { return otherY; }
            }

            /// <summary>The seconds since the two met, in whole steps of the cycle: 0 in OnMeet.</summary>
            [LocalSafe]
            public float Since
            {
                get { return (Tracking.StepNow - met) * Interactions.Step; }
            }

            /// <summary>The map's own value for this pair, kept until it parts; null until set.</summary>
            [LocalSafe]
            public object? Data
            {
                get { return data; }
                set { data = value; }
            }

            /// <summary>The seconds left of the cooldown TryCooldown started; 0 when none runs.</summary>
            [LocalSafe]
            public float Cooldown
            {
                get
                {
                    var left = coolUntil - Tracking.StepNow;
                    return left > 0 ? left * Interactions.Step : 0.0f;
                }
            }

            /// <summary>Whether the pair is paused: it does not tick, and it still parts.</summary>
            [LocalSafe]
            public bool IsPaused
            {
                get { return paused; }
            }

#if DEBUG
            /// <summary>The seconds left of a Wait, for the debug mode; 0 when none runs.</summary>
            internal float Waits
            {
                get
                {
                    var left = waitUntil - Tracking.StepNow;
                    return left > 0 ? left * Interactions.Step : 0.0f;
                }
            }

#endif
            /// <summary>Whether the pair ticks at this step: it is not paused and does not wait.</summary>
            internal bool Ready
            {
                get { return !paused && (Tracking.cycle ?? Tracking.Beat).StepNow >= waitUntil; }
            }

            /// <summary>True, and starts a cooldown of the seconds, rounded up to whole steps, when none runs; false while one does. For "at most once a second" with a fast tick.</summary>
            public bool TryCooldown(float seconds)
            {
                Dev.Check(seconds >= 0.0f, "Pair.TryCooldown: seconds must be 0 or more");
                var now = Tracking.StepNow;
                if (now < coolUntil)
                {
                    return false;
                }

                coolUntil = now + StepsOf(seconds);
                return true;
            }

            /// <summary>Skips this pair's OnTick for the seconds, rounded up to whole steps: it ticks again at the first tick after them. The pair stays, and still parts.</summary>
            public void Wait(float seconds)
            {
                Dev.Check(seconds >= 0.0f, "Pair.Wait: seconds must be 0 or more");
                waitUntil = Tracking.StepNow + StepsOf(seconds);
            }

            /// <summary>Stops the pair's OnTick until Resume; the pair stays, and still parts.</summary>
            public void Pause()
            {
                paused = true;
            }

            /// <summary>Lets the pair tick again from the next tick.</summary>
            public void Resume()
            {
                paused = false;
            }

            /// <summary>Parts the pair now (OnPart runs), and the two do not meet again until the other has been out of range at a check. To leave an other out for good, Accepts says no.</summary>
            public void Stop()
            {
                if (Gone(this))
                {
                    return;
                }

                interaction.Stopping(otherEntry);
                End();
            }

            /// <summary>Sets up a new pair: its interaction, its two sides, the step they met at, and the places of this check.</summary>
            internal void Begin(Interaction of, THolder holderThing, TOther otherThing, Entry entry, int check, float hx, float hy)
            {
                interaction = of;
                holder = holderThing;
                other = otherThing;
                otherEntry = entry;
                met = Tracking.StepNow;
                Keep(check, hx, hy, entry.X, entry.Y);
            }

            /// <summary>Marks the pair kept at a check, with the places read at it.</summary>
            internal void Keep(int check, float hx, float hy, float ox, float oy)
            {
                kept = check;
                holderX = hx;
                holderY = hy;
                otherX = ox;
                otherY = oy;
            }

            /// <summary>Takes the pair out of its interaction's lists and its other's, then runs OnPart; what the pair made ends after.</summary>
            protected sealed override void OnEnd()
            {
                interaction.Unpair(this, otherEntry);
                interaction.RunPart(this);
            }

            /// <summary>Whole steps of the cycle for seconds, rounded up; 0 for none.</summary>
            private static int StepsOf(float seconds)
            {
                return seconds <= 0.0f ? 0 : Cycle.Steps(seconds);
            }
        }
    }
}
