using System;
using System.Collections.Generic;

namespace Moonwell.Engine
{
    /// <summary>
    /// A missile's contacts with units, destructables, items, other missiles and walls. At Create the package starts one
    /// interaction on the missile for each of the first four kinds whose setting is not None, belonging to the missile. Its
    /// range covers the missile's ball, the largest thing of the kind met so far and the missile's move this step (for
    /// missiles, the farthest any missile moved too). Each check of a pair sweeps the ball along the step's move, from where
    /// it began to where it is, against the other's upright cylinder where the other is now: the flat circle first, the
    /// heights only when that meets, so a fast missile cannot skip a footman between two steps. Two missiles sweep their
    /// balls against each other along both moves, and meet once, through the one made first. Walls are not found through the
    /// grid, which keys a thing by its middle: the pass runs each missile with a WallContact down the package's list of walls
    /// right after its move, sweeping its ball against each wall's upright strip. Apart, a pair waits for the earliest time
    /// they could touch, from their distance and the missile's speed (and a unit's fastest walk, or the other missile's
    /// speed). At a contact, in order: CanHit (refused, nothing, and the pair is not checked again until it parts); OnImpact
    /// on the missile and its components (for two missiles, on both, the earlier first); for a unit, OnStruck on its class,
    /// components and buffs; the damage; the outcome. The step's checks make no table; an Impact is made at a contact only.
    /// </summary>
    internal static class Contacts
    {
        /// <summary>The pair goes on: it is checked again at its next tick.</summary>
        internal const int Go = 0;

        /// <summary>The pair is paused until it parts: it was refused, or it reacted as a Pass.</summary>
        internal const int Hold = 1;

        /// <summary>The pair waits for the seconds given before its next check.</summary>
        internal const int Later = 2;

        /// <summary>No contact came of it, or a Pass reacted: the ball goes on to the end of its move, so a wall further along may still be met. A pair takes it as Go.</summary>
        internal const int Through = 3;

        /// <summary>The farthest a missile's ball reaches in a step, its radius plus its move, taken before a farther one is seen.</summary>
        private const float FirstMissile = 16.0f;

        /// <summary>A wall's box, widened by the ball's radius and the Skin, does not hold the ball's move: passed over.</summary>
        private const int Far = 0;

        /// <summary>The move is in the wall's box, but the ball does not meet the wall along it.</summary>
        private const int Off = 1;

        /// <summary>The ball meets the wall along its move.</summary>
        private const int Met = 2;

        /// <summary>How far beyond its radius from a wall a ball is still taken to be at it, so one sliding along it, which float rounding may carry just out and back in, does not meet it again.</summary>
        private const float Skin = 1.0f;

        /// <summary>The largest radius of a unit taken before a larger one is met.</summary>
        private const float FirstUnit = 64.0f;

        /// <summary>The fastest a unit walks, in game units a second: what a unit may add to the speed a missile closes on it at.</summary>
        private const float Walk = 522.0f;

        /// <summary>The bit of OnStruck in a class's mask.</summary>
        private const int StruckBit = 1;

        /// <summary>A length shorter than this gives no direction, for a normal.</summary>
        private const float Tiny = 0.0001f;

        /// <summary>The least a ball's middle on or past a wall's line is put back out from it, so a ball of radius 0 is on its own side, clear of a float's last bit at a map's widest.</summary>
        private const float Clear = 0.01f;

        /// <summary>A time past either end of every step, for a sweep that stays inside the round part or the heights.</summary>
        private const float Always = 1000000.0f;

        /// <summary>A Bounce or Slide that comes back to a thing's top slower than this (plus a step's gravity) soon after it touched it goes on along the top with no impact, as the ground's slow bounce does.</summary>
        private const float Settle = 5.0f;

        /// <summary>How many steps after a pair last touched a ball coming back onto the thing's top counts as settling on it.</summary>
        private const int Recent = 4;

        /// <summary>The largest radius of a unit met so far, at least FirstUnit; the unit checks' range covers it.</summary>
        private static float largestUnit;

        /// <summary>The largest radius of a destructable met so far, at least the default's.</summary>
        private static float largestDestructable;

        /// <summary>The largest radius of an item: they are all the same.</summary>
        private static float largestItem;

        /// <summary>The farthest any missile has reached in a step since a missile first started missile checks, its radius plus its move, at least FirstMissile; the missile checks' range covers it.</summary>
        private static float largestMissile;

        /// <summary>Whether any missile has started checks against other missiles since the install, so the pass sets their range each step and every missile's reach counts toward the farthest.</summary>
        internal static bool meeting;

        /// <summary>A count of the walks down the walls for a missile, so a wall met in this walk is not met again in it.</summary>
        private static int wallRound;

        /// <summary>The reactions each class writes, keyed by the class (never by an actor); made on first use.</summary>
        private static Dictionary<object, int>? masks;

        /// <summary>Forgets the largest radii met and the classes' masks; an install starts afresh.</summary>
        internal static void Reset()
        {
            largestUnit = FirstUnit;
            largestDestructable = Shapes.DestructableRadius;
            largestItem = Shapes.ItemRadius;
            largestMissile = FirstMissile;
            meeting = false;
            wallRound = 0;
            masks = null;
        }

        /// <summary>Starts a new missile's checks: one interaction for each kind whose setting is not None; walls need none.</summary>
        internal static void Begin(Missile missile)
        {
            if (missile.missileContact != Contact.None)
            {
                Start(missile, ImpactKind.Missile);
            }

            if (missile.unitContact != Contact.None)
            {
                Start(missile, ImpactKind.Unit);
            }

            if (missile.destructableContact != Contact.None)
            {
                Start(missile, ImpactKind.Destructable);
            }

            if (missile.itemContact != Contact.None)
            {
                Start(missile, ImpactKind.Item);
            }
        }

        /// <summary>A setting changed: a created missile that has not begun to end starts the checks of a kind set to other than None, if they have not started; the checks of a kind set to None pause, and those set back resume unless it rests. WallContact set to None forgets the walls it was at.</summary>
        internal static void Changed(Missile missile)
        {
            if (missile.entry == null || missile.ending || missile.HasEnded)
            {
                return;
            }

            Begin(missile);
            var resting = missile.resting;
            Turn(missile.unitChecks, missile.unitContact, resting);
            Turn(missile.destructableChecks, missile.destructableContact, resting);
            Turn(missile.itemChecks, missile.itemContact, resting);
            Turn(missile.missileChecks, missile.missileContact, resting);
            if (missile.wallContact == Contact.None && missile.wallTouches != null)
            {
                Raw.Truncate(missile.wallTouches, 0);
            }
        }

        /// <summary>Pauses a missile's checks, as it comes to rest.</summary>
        internal static void Pause(Missile missile)
        {
            if (missile.unitChecks != null)
            {
                missile.unitChecks.Pause();
            }

            if (missile.destructableChecks != null)
            {
                missile.destructableChecks.Pause();
            }

            if (missile.itemChecks != null)
            {
                missile.itemChecks.Pause();
            }

            if (missile.missileChecks != null)
            {
                missile.missileChecks.Pause();
            }
        }

        /// <summary>Resumes a missile's checks of the kinds whose setting is not None, as it wakes from rest; the next comes at the next step.</summary>
        internal static void Resume(Missile missile)
        {
            Turn(missile.unitChecks, missile.unitContact, false);
            Turn(missile.destructableChecks, missile.destructableContact, false);
            Turn(missile.itemChecks, missile.itemContact, false);
            Turn(missile.missileChecks, missile.missileContact, false);
        }

        /// <summary>Pauses checks whose kind is set to None, so they cost nothing; resumes paused ones whose kind is not, unless the missile rests.</summary>
        private static void Turn(Interaction? checks, Contact setting, bool resting)
        {
            if (checks == null)
            {
                return;
            }

            if (setting == Contact.None)
            {
                checks.Pause();
            }
            else if (!resting && checks.IsPaused)
            {
                checks.Resume();
            }
        }

        /// <summary>Keeps where the missile is now as where its flight or its contacts last put it; a check that finds it elsewhere knows it was moved from outside, and does not sweep it along the jump.</summary>
        private static void Placed(Missile missile, float x, float y, float z)
        {
            missile.placedX = x;
            missile.placedY = y;
            missile.placedZ = z;
        }

        /// <summary>Keeps where the missile's step put it, and sets the range of its checks: its radius, the largest thing of the kind met so far, and how far it moved this step along the ground, so the thing its move swept is in range where it ended.</summary>
        internal static void Reach(Missile missile)
        {
            var entry = missile.entry!;
            Placed(missile, entry.X, entry.Y, missile.z);
            var units = missile.unitChecks;
            var destructables = missile.destructableChecks;
            var items = missile.itemChecks;
            var missiles = missile.missileChecks;
            var none = units == null && destructables == null && items == null && missiles == null;
            if (none && !meeting)
            {
                return;
            }

            var dx = entry.X - missile.lastX;
            var dy = entry.Y - missile.lastY;
            var reach = missile.radius + Trig.Sqrt(dx * dx + dy * dy);
            if (meeting)
            {
                // Every missile counts toward the farthest, as one with no missile checks is still met by those that have
                // them; a missile's own range is set once every missile has moved.
                missile.meetReach = reach;
                if (reach > largestMissile)
                {
                    largestMissile = reach;
                }
            }

            if (none)
            {
                return;
            }

            if (units != null)
            {
                units.range = reach + largestUnit;
            }

            if (destructables != null)
            {
                destructables.range = reach + LargestDestructable();
            }

            if (items != null)
            {
                items.range = reach + largestItem;
            }
        }

        /// <summary>Sets the range of the checks against other missiles of the missiles that were in the pass's list at the step's start and have not begun to end: their own reach this step plus the farthest any missile reached, so a pair of missiles that closed on each other this step is in range of both.</summary>
        internal static void MeetRanges(List<Missile> list, int count)
        {
            var farthest = largestMissile;
            for (var i = 0; i < count; i++)
            {
                var missile = Raw.At(list, i);
                var checks = missile.missileChecks;
                if (checks != null && !missile.ending)
                {
                    checks.range = missile.meetReach + farthest;
                }
            }
        }

        /// <summary>A unit has met a missile's checks: its radius, kept for the pair, raises the largest unit's if larger.</summary>
        internal static Touch MetUnit(Unit unit)
        {
            var touch = new Touch();
            touch.radius = Shapes.UnitRadius(unit);
            if (touch.radius > largestUnit)
            {
                largestUnit = touch.radius;
            }

            return touch;
        }

        /// <summary>A destructable has met a missile's checks: its size, kept for the pair, raises the largest destructable's if larger.</summary>
        internal static Touch MetDestructable(Destructable destructable)
        {
            var touch = new Touch();
            float radius;
            float height;
            Shapes.DestructableSize(destructable, out radius, out height);
            touch.radius = radius;
            touch.height = height;
            if (touch.radius > largestDestructable)
            {
                largestDestructable = touch.radius;
            }

            return touch;
        }

        /// <summary>An item has met a missile's checks: its size, kept for the pair.</summary>
        internal static Touch MetItem()
        {
            var touch = new Touch();
            touch.radius = Shapes.ItemRadius;
            touch.height = Shapes.ItemHeight;
            return touch;
        }

        /// <summary>A check of a missile and a unit at its place this check: its velocity is worked out from its place at the pair's last check, then the sweep. What the pair does next; the seconds to wait for Later.</summary>
        internal static int UnitTick(Missile missile, Unit unit, Touch touch, float x, float y, out float wait)
        {
            var now = Tracking.StepNow;
            var vx = 0.0f;
            var vy = 0.0f;
            if (touch.seen && now > touch.step)
            {
                var seconds = (now - touch.step) * Interactions.Step;
                vx = (x - touch.x) / seconds;
                vy = (y - touch.y) / seconds;
            }

            touch.seen = true;
            touch.step = now;
            touch.x = x;
            touch.y = y;
            return Check(missile, touch, ImpactKind.Unit, unit, unit, x, y, vx, vy, Walk, missile.unitContact, out wait);
        }

        /// <summary>A check of a missile and a destructable at its place. What the pair does next; the seconds to wait for Later.</summary>
        internal static int DestructableTick(Missile missile, Destructable destructable, Touch touch, float x, float y, out float wait)
        {
            return Check(missile, touch, ImpactKind.Destructable, destructable, null, x, y, 0.0f, 0.0f, 0.0f, missile.destructableContact, out wait);
        }

        /// <summary>A check of a missile and an item at its place. What the pair does next; the seconds to wait for Later.</summary>
        internal static int ItemTick(Missile missile, Item item, Touch touch, float x, float y, out float wait)
        {
            return Check(missile, touch, ImpactKind.Item, item, null, x, y, 0.0f, 0.0f, 0.0f, missile.itemContact, out wait);
        }

        /// <summary>Starts a missile's checks of a kind, unless they have started: an interaction belonging to the missile, paused while it rests.</summary>
        private static void Start(Missile missile, ImpactKind kind)
        {
            Interaction checks;
            if (kind == ImpactKind.Unit)
            {
                if (missile.unitChecks != null)
                {
                    return;
                }

                checks = new UnitContacts();
                checks.range = missile.radius + largestUnit;
                missile.unitChecks = checks;
            }
            else if (kind == ImpactKind.Destructable)
            {
                if (missile.destructableChecks != null)
                {
                    return;
                }

                checks = new DestructableContacts();
                checks.range = missile.radius + LargestDestructable();
                missile.destructableChecks = checks;
            }
            else if (kind == ImpactKind.Item)
            {
                if (missile.itemChecks != null)
                {
                    return;
                }

                checks = new ItemContacts();
                checks.range = missile.radius + largestItem;
                missile.itemChecks = checks;
            }
            else
            {
                if (missile.missileChecks != null)
                {
                    return;
                }

                checks = new MissileContacts();
                checks.range = missile.radius + largestMissile;
                missile.missileChecks = checks;
                meeting = true;
            }

            missile.Interact(checks, missile);
            if (missile.resting)
            {
                checks.Pause();
            }
        }

        /// <summary>The largest radius of a destructable: of those met, the default and those Physics.DestructableSize gave.</summary>
        private static float LargestDestructable()
        {
            var sized = Physics.Current().LargestDestructable;
            return sized > largestDestructable ? sized : largestDestructable;
        }

        /// <summary>
        /// The sweep of a missile's ball along its move this step against a thing's upright cylinder, and the contact when they
        /// meet. A ball that began the step inside the cylinder meets it at once, unless it touched it at the last check and is
        /// not moving into it. Apart, the pair waits until they could touch. What the pair does next; the seconds to wait for
        /// Later.
        /// </summary>
        private static int Check(Missile missile, Touch touch, ImpactKind kind, Actor other, Unit? unit, float cx, float cy, float otherVX, float otherVY, float closing, Contact setting, out float wait)
        {
            wait = 0.0f;
            if (setting == Contact.None || missile.ending)
            {
                // Set to None in this check: the checks pause, and this pair waits until they part.
                Leave(touch);
                return Hold;
            }

            Moved(missile);
            var entry = missile.entry!;
            var bx = entry.X;
            var by = entry.Y;
            var bz = missile.z;
            var radius = missile.radius;
            var ax = missile.lastX;
            var ay = missile.lastY;
            var az = missile.lastZ;
            var dx = bx - ax;
            var dy = by - ay;
            var dz = bz - az;
            var reach = radius + touch.radius;
            var px = ax - cx;
            var py = ay - cy;

            // The round part: when along the move the ball's middle is within reach of the cylinder's axis, flat.
            float t0;
            float t1;
            var flat = dx * dx + dy * dy;
            var gap = px * px + py * py - reach * reach;
            if (flat <= 0.0f)
            {
                if (gap > 0.0f)
                {
                    return Apart(missile, touch, bx - cx, by - cy, reach, closing, 0.0f, out wait);
                }

                t0 = -Always;
                t1 = Always;
            }
            else
            {
                var half = px * dx + py * dy;
                var square = half * half - flat * gap;
                if (square < 0.0f)
                {
                    return Apart(missile, touch, bx - cx, by - cy, reach, closing, 0.0f, out wait);
                }

                var root = Trig.Sqrt(square);
                t0 = (-half - root) / flat;
                t1 = (-half + root) / flat;
                if (t1 < 0.0f || t0 > 1.0f)
                {
                    return Apart(missile, touch, bx - cx, by - cy, reach, closing, 0.0f, out wait);
                }
            }

            // The heights, read only now: from the ground (plus a unit's fly height), the ball's radius beyond each end.
            float bottom;
            float height;
            if (unit != null)
            {
                bottom = Ground.Height(cx, cy) + unit.FlyHeight;
                height = Shapes.UnitHeight(unit);
            }
            else
            {
                bottom = Ground.Height(cx, cy);
                height = touch.height;
            }

            var top = bottom + height + radius;
            bottom -= radius;
            float z0;
            float z1;
            if (dz == 0.0f)
            {
                if (az < bottom || az > top)
                {
                    Leave(touch);
                    return Go;
                }

                z0 = -Always;
                z1 = Always;
            }
            else
            {
                z0 = (bottom - az) / dz;
                z1 = (top - az) / dz;
                if (z0 > z1)
                {
                    var swap = z0;
                    z0 = z1;
                    z1 = swap;
                }
            }

            var enter = t0 > z0 ? t0 : z0;
            var exit = t1 < z1 ? t1 : z1;
            if (enter > exit || exit < 0.0f || enter > 1.0f)
            {
                Leave(touch);
                return Go;
            }

            if (touch.passed)
            {
                // A Pass reacted on entering and has not left the thing since.
                touch.touching = true;
                return Go;
            }

            // The normal out of the cylinder where the ball met it; for a ball that began inside, out of the nearest face.
            var inside = enter < 0.0f;
            var t = inside ? 0.0f : enter;
            float nx;
            float ny;
            float nz;
            if (inside)
            {
                var off = Trig.Sqrt(px * px + py * py);
                var side = reach - off;
                var above = top - az;
                var below = az - bottom;
                if (above <= side && above <= below)
                {
                    nx = 0.0f;
                    ny = 0.0f;
                    nz = 1.0f;
                }
                else if (below <= side)
                {
                    nx = 0.0f;
                    ny = 0.0f;
                    nz = -1.0f;
                }
                else
                {
                    Side(missile, px, py, off, out nx, out ny, out nz);
                }
            }
            else if (t0 >= z0)
            {
                var hx = px + t * dx;
                var hy = py + t * dy;
                Side(missile, hx, hy, Trig.Sqrt(hx * hx + hy * hy), out nx, out ny, out nz);
            }
            else
            {
                nx = 0.0f;
                ny = 0.0f;
                nz = dz < 0.0f ? 1.0f : -1.0f;
            }

            return Hit(missile, touch, kind, other, unit, t, inside, nx, ny, nz, top, otherVX, otherVY, setting, 1.0f);
        }

        /// <summary>
        /// A missile's ball meets a thing along its move this step, a share t of the way (0 when it began inside it), with the
        /// normal out of the thing there, the thing's top (for a ball settling on it) and the thing's flat velocity. The ball
        /// is put where it met the thing, at the velocity it met it at; then CanHit, OnImpact on the missile and its
        /// components, OnStruck on a unit, the damage and the outcome. Bounce keeps the missile's Bounciness times the
        /// surface's share (a wall's Bounciness, else 1); Slide on a wall bounces so too, and once it meets it slower than
        /// Settle loses the speed into it and slides along it, keeping the rest. What the
        /// pair does next: Hold when refused, Through when no contact came of it, it passed or it went on along a wall, else
        /// Go.
        /// </summary>
        private static int Hit(Missile missile, Touch touch, ImpactKind kind, Actor other, Unit? unit, float t, bool inside, float nx, float ny, float nz, float top, float otherVX, float otherVY, Contact setting, float surface)
        {
            var entry = missile.entry!;
            var radius = missile.radius;
            var ax = missile.lastX;
            var ay = missile.lastY;
            var az = missile.lastZ;
            var bx = entry.X;
            var by = entry.Y;
            var bz = missile.z;
            var dx = bx - ax;
            var dy = by - ay;
            var dz = bz - az;

            // The velocity where it met the thing: gravity has acted only for the part of the step before.
            var vx = missile.vx;
            var vy = missile.vy;
            // Only a move that was a free fall took the step's gravity whole: one that met the ground or rolled along it this
            // step has the ground's velocity, and none is given back.
            var late = (dx * dx + dy * dy > 0.0f || dz != 0.0f) && missile.fell ? missile.gravity * Interactions.Current().Step * (1.0f - t) : 0.0f;
            var vz = missile.vz + late;
            var into = (vx - otherVX) * nx + (vy - otherVY) * ny + vz * nz;
            var now = Tracking.StepNow;
            var grid = Interactions.Grid;
            var world = Physics.World;
            if (nz > 0.0f && bz < top && (setting == Contact.Bounce || setting == Contact.Slide) && touch.contacted && now - touch.lastTouch <= Recent && into > -(Settle + missile.gravity * Interactions.Current().Step))
            {
                // Back on the thing's top soon after touching it, slowly: it goes on along the top with no impact, and may
                // roll off it.
                touch.touching = true;
                touch.lastTouch = now;
                missile.vx = vx;
                missile.vy = vy;
                missile.vz = into < 0.0f ? vz - into : vz;
                grid.Move(entry, bx, by);
                missile.z = top;
                Placed(missile, bx, by, top);
                Flight.Place(missile, world, bx, by, top);
                return Go;
            }

            if (inside && touch.touching && into >= 0.0f)
            {
                // It touched the thing at the last check and is not moving into it: it is leaving, as after a bounce.
                return Through;
            }

            var wall = touch.wall;
            if (wall != null && nz == 0.0f && into < 0.0f && touch.contacted && now - touch.lastTouch <= Recent && (setting == Contact.Bounce || setting == Contact.Slide) && -into < Settle + (missile.rolling ? Physics.Current().Gravity : missile.gravity) * Interactions.Current().Step)
            {
                // Back at a wall's side soon after touching it, slowly, as a slope pulls a rolling ball into it at every
                // step: it goes on along the side with no impact, to the end of its move put back out at the side, as the
                // guide's loop goes on after a hit for the rest of the step. Left where it met the wall, it would meet it
                // again at the start of the next step, and stand there. The walk goes on to the walls further along its
                // move (Through): a second wall closing a corner ahead is met this step, not a step late from its far side.
                touch.touching = true;
                touch.lastTouch = now;
                missile.vx = vx - into * nx;
                missile.vy = vy - into * ny;
                Beside(wall, radius, nx, ny, ref bx, ref by);
                grid.Move(entry, bx, by);
                Placed(missile, bx, by, bz);
                Flight.Place(missile, world, bx, by, bz);
                return Through;
            }

            touch.touching = true;
            touch.contacted = true;
            touch.lastTouch = now;
            if (!Asks(missile, other))
            {
                return Hold;
            }

            // It is put where it met the thing for the reactions, at the velocity it met it at.
            var hitX = ax + t * dx;
            var hitY = ay + t * dy;
            var hitZ = az + t * dz;
            grid.Move(entry, hitX, hitY);
            missile.z = hitZ;
            missile.vx = vx;
            missile.vy = vy;
            missile.vz = vz;
            var pointX = hitX - radius * nx;
            var pointY = hitY - radius * ny;
            var pointZ = hitZ - radius * nz;
            var impact = new Impact(kind, other, pointX, pointY, pointZ, nx, ny, nz, into < 0.0f ? -into : 0.0f, setting, missile.impactDamage);
            Flight.Impacted(missile, impact);
            if (Ended(missile))
            {
                return Go;
            }

            if (unit != null)
            {
                Struck(missile, unit, impact);
                if (Ended(missile))
                {
                    return Go;
                }
            }

            Deal(missile, impact, unit, other);
            if (Ended(missile))
            {
                return Go;
            }

            // The outcome, from the velocity the reactions left; a reaction that moved it leaves it where it put it.
            vx = missile.vx;
            vy = missile.vy;
            vz = missile.vz;
            into = (vx - otherVX) * nx + (vy - otherVY) * ny + vz * nz;
            var outcome = impact.contact;
            var moved = entry.X != hitX || entry.Y != hitY || missile.z != hitZ;
            if (outcome == Contact.Stop)
            {
                if (unit != null && into < 0.0f)
                {
                    // A Stop with mass shoves as a bounce that keeps nothing.
                    Shove(missile, unit, nx, ny, nz, into, 0.0f);
                    if (Ended(missile))
                    {
                        return Go;
                    }
                }

                StopAt(missile, entry, pointX, pointY, pointZ);
                return Go;
            }

            if (outcome == Contact.Pass || outcome == Contact.None)
            {
                // It does not react to the thing again until it has left it.
                touch.passed = true;
                FlyOn(missile, entry, late, moved, bx, by, bz);
                return Through;
            }

            // Bounce, and Slide, which is Bounce off anything but the ground and walls; Slide on a wall bounces until it meets
            // it slower than Settle, then slides along it. A bounce that leaves it faster ends the waits on it.
            if (into < 0.0f)
            {
                var before = vx * vx + vy * vy + vz * vz;
                if (kind == ImpactKind.Wall && outcome == Contact.Slide && -into < Settle)
                {
                    missile.vx = vx - into * nx;
                    missile.vy = vy - into * ny;
                    missile.vz = vz - into * nz;
                }
                else
                {
                    var keep = missile.bounciness * surface;
                    if (unit == null || !Shove(missile, unit, nx, ny, nz, into, keep))
                    {
                        var back = (1.0f + keep) * into;
                        missile.vx = vx - back * nx;
                        missile.vy = vy - back * ny;
                        missile.vz = vz - back * nz;
                    }
                }

                if (Ended(missile))
                {
                    return Go;
                }

                Quicker(missile, before);
            }

            if (wall != null && !moved && radius < Clear)
            {
                // A ball thinner than Clear met the wall with its middle on the wall's line, or a float's last bit past it:
                // it stays just off it on the side it came from, so the next step's sweep knows its side.
                Beside(wall, radius, nx, ny, ref hitX, ref hitY);
                grid.Move(entry, hitX, hitY);
            }

            Stay(missile, entry, moved, hitX, hitY, hitZ);
            return Go;
        }

        /// <summary>A Stop: the missile is put at the point of contact, its effects with it, its ImpactModel and ImpactSound played there, and it ends.</summary>
        private static void StopAt(Missile missile, Entry entry, float x, float y, float z)
        {
            var world = Physics.World;
            Interactions.Grid.Move(entry, x, y);
            missile.z = z;
            Flight.Place(missile, world, x, y, z);
            Flight.Burst(missile, world, x, y, z);
            missile.End();
        }

        /// <summary>A Pass or None: the missile flies on from where its move took it, as it was, gravity acting on for the rest of the step (the late part taken back); a reaction that moved it leaves it where it was put.</summary>
        private static void FlyOn(Missile missile, Entry entry, float late, bool moved, float bx, float by, float bz)
        {
            missile.vz -= late;
            if (moved)
            {
                Jumped(missile, entry);
                return;
            }

            Interactions.Grid.Move(entry, bx, by);
            missile.z = bz;
            Placed(missile, bx, by, bz);
        }

        /// <summary>A bounce: the missile stays where it met the thing, its effects with it, and goes on from there at the next step; a reaction that moved it leaves it where it was put.</summary>
        private static void Stay(Missile missile, Entry entry, bool moved, float hitX, float hitY, float hitZ)
        {
            if (moved)
            {
                Jumped(missile, entry);
                return;
            }

            Placed(missile, hitX, hitY, hitZ);
            Flight.Place(missile, Physics.World, hitX, hitY, hitZ);
        }

        /// <summary>Starts a check of a missile: one found away from where its flight or its contacts last put it was moved from outside since (its X, Y or Z set), and is not swept along the jump.</summary>
        private static void Moved(Missile missile)
        {
            var entry = missile.entry!;
            var x = entry.X;
            var y = entry.Y;
            var z = missile.z;
            if (x != missile.placedX || y != missile.placedY || z != missile.placedZ)
            {
                missile.lastX = x;
                missile.lastY = y;
                missile.lastZ = z;
                missile.lastStep = Tracking.StepNow;
                var walls = missile.wallTouches;
                if (walls != null)
                {
                    // It is no longer at the walls it was at.
                    Raw.Truncate(walls, 0);
                }

                Placed(missile, x, y, z);
            }
        }

        /// <summary>A reaction moved the missile at a contact: it stays where it was put, and its next sweep starts there. It is no longer at the walls it was at, so it forgets them, as when it is moved from outside: a wall it passed or refused is met again where it next comes to it.</summary>
        private static void Jumped(Missile missile, Entry entry)
        {
            var x = entry.X;
            var y = entry.Y;
            var z = missile.z;
            missile.lastX = x;
            missile.lastY = y;
            missile.lastZ = z;
            missile.lastStep = Tracking.StepNow;
            var walls = missile.wallTouches;
            if (walls != null)
            {
                Raw.Truncate(walls, 0);
            }

            Placed(missile, x, y, z);
        }

        /// <summary>The ball is not touching the thing: a Pass that reacted to it may react again when it next enters, and a refusal is asked again.</summary>
        private static void Leave(Touch touch)
        {
            touch.touching = false;
            touch.passed = false;
            touch.held = false;
        }

        /// <summary>The two are apart along the move: not touching, and the pair waits until they could touch, from their distance now and how fast they could close.</summary>
        private static int Apart(Missile missile, Touch touch, float ox, float oy, float reach, float closing, float quicker, out float wait)
        {
            Leave(touch);
            wait = Until(missile, ox, oy, reach, closing, quicker);
            return wait > 0.0f ? Later : Go;
        }

        /// <summary>
        /// The earliest seconds from now a missile could touch a thing whose middle is (ox, oy) from it, flat: the gap between
        /// them, (distance - reach), over the missile's speed plus how fast the thing could close (a unit's fastest walk,
        /// another missile's speed); with gravity or an Acceleration the missile may speed up, and the thing by quicker a
        /// second, which the time allows for. 0 when they could touch at once, or never close.
        /// </summary>
        private static float Until(Missile missile, float ox, float oy, float reach, float closing, float quicker)
        {
            var gap = Trig.Sqrt(ox * ox + oy * oy) - reach;
            if (gap <= 0.0f)
            {
                return 0.0f;
            }

            var vx = missile.vx;
            var vy = missile.vy;
            var vz = missile.vz;
            var speed = Trig.Sqrt(vx * vx + vy * vy + vz * vz) + closing;
            var gravity = missile.gravity;
            var faster = (gravity < 0.0f ? -gravity : gravity) + (missile.acceleration > 0.0f ? missile.acceleration : 0.0f) + quicker;
            if (faster > 0.0f)
            {
                // The time to cover the gap from speed, speeding up at faster: gap = speed t + faster t^2 / 2.
                return 2.0f * gap / (speed + Trig.Sqrt(speed * speed + 2.0f * faster * gap));
            }

            return speed > 0.0f ? gap / speed : 0.0f;
        }

        /// <summary>The level normal out of a cylinder's side toward a flat offset from its axis; at the axis itself, where there is none, against the missile's flight along the ground, else against its fall or rise, else up.</summary>
        private static void Side(Missile missile, float hx, float hy, float length, out float nx, out float ny, out float nz)
        {
            if (length > Tiny)
            {
                nx = hx / length;
                ny = hy / length;
                nz = 0.0f;
                return;
            }

            var vx = missile.vx;
            var vy = missile.vy;
            var along = Trig.Sqrt(vx * vx + vy * vy);
            if (along > Tiny)
            {
                nx = -vx / along;
                ny = -vy / along;
                nz = 0.0f;
                return;
            }

            nx = 0.0f;
            ny = 0.0f;
            nz = missile.vz > 0.0f ? -1.0f : 1.0f;
        }

        /// <summary>
        /// Shares the closing speed along the normal between a missile with mass and a unit that can be pushed, by their
        /// masses, keeping a share of it (the missile's Bounciness, or nothing for a Stop): the missile's velocity changes by
        /// its part, and the unit gets its part through a push, its pusher the missile's source. False, and nothing changes,
        /// when the missile has no mass or the unit cannot be moved: a structure, an immovable Body, a mass of 0.
        /// </summary>
        private static bool Shove(Missile missile, Unit unit, float nx, float ny, float nz, float into, float keep)
        {
            var mass = missile.mass;
            if (mass <= 0.0f || unit.HasEnded || unit.IsGone)
            {
                return false;
            }

            var world = Physics.World;
            var body = unit.Get<Body>();
            if (body != null && body.HasEnded)
            {
                body = null;
            }

            if ((body != null && body.Immovable) || world.IsStructure(unit.Handle))
            {
                return false;
            }

            var weight = body != null ? body.Mass : world.CollisionSize(unit.Handle);
            if (weight <= 0.0f)
            {
                return false;
            }

            var share = -(1.0f + keep) * into / (1.0f / mass + 1.0f / weight);
            var own = share / mass;
            missile.vx += own * nx;
            missile.vy += own * ny;
            missile.vz += own * nz;
            var given = share / weight;
            Pushes.Push(unit, -given * nx, -given * ny, -given * nz, missile.source);
            return true;
        }

        /// <summary>Asks the missile's CanHit about a thing, with the missile as the current owner; an error it throws is reported and refuses.</summary>
        private static bool Asks(Missile missile, Actor other)
        {
            var previous = Owner.current;
            Owner.current = missile;
            bool answer;
            try
            {
                answer = missile.CanHit(other);
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(missile) + ".CanHit", error);
                answer = false;
            }

            Owner.current = previous;
            return answer;
        }

        /// <summary>Runs OnStruck on a struck unit's class, then its components and buffs in the order they were added, with the missile's owner as the current owner; only where the class writes it, not on what has ended, and none once the missile has ended. Each is guarded.</summary>
        private static void Struck(Missile missile, Unit unit, Impact impact)
        {
            var previous = Owner.current;
            Owner.current = missile.Owner ?? Game.Instance;
            if (!unit.HasEnded && (MaskOf(unit) & StruckBit) != 0)
            {
                Call(unit, impact);
            }

            var count = unit.BeginDispatch();
            for (var i = 0; i < count && !Ended(missile); i++)
            {
                var component = unit.ComponentAt(i) as UnitComponent;
                if (component != null && !component.HasEnded && (MaskOf(component) & StruckBit) != 0)
                {
                    Call(component, impact);
                }
            }

            unit.EndDispatch();
            Owner.current = previous;
        }

        /// <summary>Calls OnStruck on an object that writes it, reporting an error it throws.</summary>
        private static void Call(object obj, Impact impact)
        {
            try
            {
                ((IMissileTarget)obj).OnStruck(impact);
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(obj) + ".OnStruck", error);
            }
        }

        /// <summary>The bits of the reactions an object's class writes, worked out once per class. Only methods the class writes count: in Lua an interface's empty defaults are not there.</summary>
        private static int MaskOf(object obj)
        {
            if (masks == null)
            {
                masks = new Dictionary<object, int>();
            }

            var key = Names.ClassOf(obj);
            if (masks.TryGetValue(key, out var mask))
            {
                return mask;
            }

            mask = 0;
            if (obj is IMissileTarget && Hooks.Overrides(obj, "OnStruck", typeof(object)))
            {
                mask |= StruckBit;
            }

            masks[key] = mask;
            return mask;
        }

        /// <summary>
        /// Deals an impact's damage, above 0, from the missile's source to the unit or destructable met, as the missile's
        /// settings say, with the missile as the tag on a unit. A source that died while the missile flew still deals it; one
        /// that has left the game deals none. With no source, none is dealt and a development build reports it. An item, a
        /// unit that has left the game, takes none.
        /// </summary>
        private static void Deal(Missile missile, Impact impact, Unit? unit, Actor other)
        {
            var amount = impact.damage;
            var destructable = other as Destructable;
            if (amount <= 0.0f || (unit == null && destructable == null))
            {
                return;
            }

            var source = missile.source;
            if (source == null)
            {
                Unsourced(missile);
                return;
            }

            if (source.HasEnded || source.IsGone)
            {
                return;
            }

            if (unit != null)
            {
                if (!unit.HasEnded && !unit.IsGone)
                {
                    source.Damage(unit, amount, missile.damageKind, missile.attackType, missile.isAttack, missile.ranged, WeaponSound.None, missile);
                }

                return;
            }

            source.Damage(destructable!, amount, missile.damageKind, missile.attackType, missile.isAttack, missile.ranged);
        }

        /// <summary>Reports, in a development build, damage at a contact of a missile that has no source; a release build leaves the check out.</summary>
        private static void Unsourced(Missile missile)
        {
            try
            {
                Dev.Check(false, Names.NameOf(missile) + ": its ImpactDamage needs a Source, so none was dealt");
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(missile) + ".ImpactDamage", error);
            }
        }

        /// <summary>
        /// A pair of two missiles has met at one of the two's checks: the one state the two keep of each other, which both
        /// sides' pairs share, so it holds whichever side checks the pair. It is found in the holder's list of what it keeps
        /// with other missiles, or made and put in both missiles' lists; the pair is noted on it as the earlier's or the
        /// later's.
        /// </summary>
        internal static Touch MetMissile(Missile holder, Missile other, Owner pair)
        {
            var early = holder.made < other.made ? holder : other;
            var late = holder.made < other.made ? other : holder;
            var touch = Shared(holder, early, late);
            if (touch == null)
            {
                touch = new Touch();
                touch.early = early;
                touch.late = late;
                Keep(holder, touch);
                Keep(other, touch);
            }

            if (holder == early)
            {
                touch.earlyPair = pair;
            }
            else
            {
                touch.latePair = pair;
            }

            return touch;
        }

        /// <summary>A pair of two missiles has parted: it is no longer noted on their shared state, and once neither side's pair is left, the state is taken out of both missiles' lists.</summary>
        internal static void PartedMissile(Missile holder, Touch touch)
        {
            if (holder == touch.early)
            {
                touch.earlyPair = null;
            }
            else
            {
                touch.latePair = null;
            }

            if (touch.earlyPair == null && touch.latePair == null)
            {
                Drop(touch.early!, touch);
                Drop(touch.late!, touch);
            }
        }

        /// <summary>What a missile keeps with an earlier and a later missile, from its list; null for none.</summary>
        private static Touch? Shared(Missile holder, Missile early, Missile late)
        {
            var touches = holder.meetTouches;
            if (touches == null)
            {
                return null;
            }

            var count = touches.Count;
            for (var i = 0; i < count; i++)
            {
                var touch = Raw.At(touches, i);
                if (touch.early == early && touch.late == late)
                {
                    return touch;
                }
            }

            return null;
        }

        /// <summary>Adds what two missiles keep of each other to the end of one's list.</summary>
        private static void Keep(Missile missile, Touch touch)
        {
            if (missile.meetTouches == null)
            {
                missile.meetTouches = new List<Touch>();
            }

            Raw.Append(missile.meetTouches, touch);
        }

        /// <summary>Takes what two missiles kept of each other out of one's list, keeping the order of the rest.</summary>
        private static void Drop(Missile missile, Touch touch)
        {
            var touches = missile.meetTouches;
            if (touches != null)
            {
                Remove(touches, touch);
            }
        }

        /// <summary>Takes a touch out of a list, keeping the order of the rest; nothing when it is not there.</summary>
        private static void Remove(List<Touch> touches, Touch touch)
        {
            var kept = 0;
            var count = touches.Count;
            for (var i = 0; i < count; i++)
            {
                var item = Raw.At(touches, i);
                if (item != touch)
                {
                    Raw.Put(touches, kept, item);
                    kept++;
                }
            }

            Raw.Truncate(touches, kept);
        }

        /// <summary>
        /// A missile's velocity was set from outside its own flight, or changed by a contact, from a speed whose square is
        /// given: when it is faster now, the pairs that wait on it (its own checks', and both sides of each missile pair it is
        /// in) check again from their next tick, as the wait was worked out from the slower speed.
        /// </summary>
        internal static void Quicker(Missile missile, float before)
        {
            var vx = missile.vx;
            var vy = missile.vy;
            var vz = missile.vz;
            if (missile.entry == null || vx * vx + vy * vy + vz * vz <= before)
            {
                return;
            }

            Unwait<Unit>(missile.unitChecks);
            Unwait<Destructable>(missile.destructableChecks);
            Unwait<Item>(missile.itemChecks);
            var touches = missile.meetTouches;
            if (touches == null)
            {
                return;
            }

            var count = touches.Count;
            for (var i = 0; i < count; i++)
            {
                var touch = Raw.At(touches, i);
                if (touch.earlyPair != null)
                {
                    Pairing.As<Missile, Missile>(touch.earlyPair).Wait(0.0f);
                }

                if (touch.latePair != null)
                {
                    Pairing.As<Missile, Missile>(touch.latePair).Wait(0.0f);
                }
            }
        }

        /// <summary>Ends the waits of the pairs of a missile's checks of a kind; nothing when they have not started.</summary>
        private static void Unwait<TOther>(Interaction? checks)
            where TOther : Actor
        {
            if (checks == null)
            {
                return;
            }

            var pairs = checks.pairs;
            var count = pairs.Count;
            for (var i = 0; i < count; i++)
            {
                Pairing.As<Missile, TOther>(Raw.At(pairs, i)).Wait(0.0f);
            }
        }

        /// <summary>
        /// A check of a missile and another missile. The one made first checks the pair when it can: the later one's check
        /// passes it over, waiting as long as the two could not touch, only while the earlier one's checks run and hold a
        /// pair with it; otherwise (the earlier one rests, its MissileContact is None, or it has not met the later one yet)
        /// the later one checks it. Both sides keep the one state. What the pair does next; the seconds to wait for Later.
        /// </summary>
        internal static int MissileTick(Missile holder, Missile other, Touch touch, out float wait)
        {
            wait = 0.0f;
            if (holder.ending || other.ending || holder.missileContact == Contact.None)
            {
                Leave(touch);
                return Go;
            }

            if (other.made < holder.made)
            {
                var theirs = other.missileChecks;
                if (theirs != null && !theirs.IsPaused && other.missileContact != Contact.None && touch.earlyPair != null)
                {
                    // The earlier one checks the pair; this side waits as long as they could not touch.
                    var holderEntry = holder.entry!;
                    var otherEntry = other.entry!;
                    wait = Closing(holder, other, holderEntry.X - otherEntry.X, holderEntry.Y - otherEntry.Y, holder.radius + other.radius);
                    return wait > 0.0f ? Later : Go;
                }

                return Meet(other, holder, touch, out wait);
            }

            return Meet(holder, other, touch, out wait);
        }

        /// <summary>
        /// Two missiles, the one made first and the other: their balls are swept against each other along both moves (the
        /// second's move taken from the first's, so they meet when the gap between their middles, moving, is the two radii),
        /// over one time frame: a missile whose move began a step earlier than the other's, as one woken from rest, has the
        /// other's move drawn back along its line to the same start. At a contact, in order: each one's CanHit about the
        /// other (either refusing, nothing, and it is not asked again until they part); each is put where they met, at the
        /// velocity it met at; OnImpact on the first and its components, then on the second, each Impact with the normal out
        /// of the other and the speed they closed at, and the outcome the missile's own MissileContact gives; then each takes
        /// its own outcome. A side whose MissileContact is None reacts to nothing and flies on, and the other meets it as a
        /// thing that does not move. Missiles take no damage. What the pair does next; the seconds to wait for Later.
        /// </summary>
        private static int Meet(Missile first, Missile second, Touch touch, out float wait)
        {
            wait = 0.0f;
            Moved(first);
            Moved(second);
            var entryA = first.entry!;
            var entryB = second.entry!;
            var radiusA = first.radius;
            var radiusB = second.radius;
            var aX0 = first.lastX;
            var aY0 = first.lastY;
            var aZ0 = first.lastZ;
            var aX1 = entryA.X;
            var aY1 = entryA.Y;
            var aZ1 = first.z;
            var bX0 = second.lastX;
            var bY0 = second.lastY;
            var bZ0 = second.lastZ;
            var bX1 = entryB.X;
            var bY1 = entryB.Y;
            var bZ1 = second.z;
            var movedA = aX1 != aX0 || aY1 != aY0 || aZ1 != aZ0;
            var movedB = bX1 != bX0 || bY1 != bY0 || bZ1 != bZ0;

            // One time frame for both: the steps since the earlier start, and the later start drawn back to it.
            var now = Tracking.StepNow;
            // A move begun in this step's pass covers one step; one kept from the step before (woken from rest), two.
            var spanA = now - first.lastStep + 1;
            var spanB = now - second.lastStep + 1;
            var frame = spanA > spanB ? spanA : spanB;
            if (frame < 1)
            {
                frame = 1;
            }

            if (movedA && spanA > 0 && spanA < frame)
            {
                var back = (float)frame / (float)spanA;
                aX0 = aX1 - (aX1 - aX0) * back;
                aY0 = aY1 - (aY1 - aY0) * back;
                aZ0 = aZ1 - (aZ1 - aZ0) * back;
            }

            if (movedB && spanB > 0 && spanB < frame)
            {
                var back = (float)frame / (float)spanB;
                bX0 = bX1 - (bX1 - bX0) * back;
                bY0 = bY1 - (bY1 - bY0) * back;
                bZ0 = bZ1 - (bZ1 - bZ0) * back;
            }

            var reach = radiusA + radiusB;

            // The first's middle from the second's, at the frame's start and along it: they touch when its length is reach.
            var px = aX0 - bX0;
            var py = aY0 - bY0;
            var pz = aZ0 - bZ0;
            var dx = aX1 - aX0 - (bX1 - bX0);
            var dy = aY1 - aY0 - (bY1 - bY0);
            var dz = aZ1 - aZ0 - (bZ1 - bZ0);
            var move = dx * dx + dy * dy + dz * dz;
            var gap = px * px + py * py + pz * pz - reach * reach;
            float t0;
            if (move <= 0.0f)
            {
                if (gap > 0.0f)
                {
                    return Parted(first, second, touch, aX1 - bX1, aY1 - bY1, reach, out wait);
                }

                t0 = -Always;
            }
            else
            {
                var half = px * dx + py * dy + pz * dz;
                var square = half * half - move * gap;
                if (square < 0.0f)
                {
                    return Parted(first, second, touch, aX1 - bX1, aY1 - bY1, reach, out wait);
                }

                var root = Trig.Sqrt(square);
                t0 = (-half - root) / move;
                var t1 = (-half + root) / move;
                if (t1 < 0.0f || t0 > 1.0f)
                {
                    return Parted(first, second, touch, aX1 - bX1, aY1 - bY1, reach, out wait);
                }
            }

            if (touch.passed || touch.held)
            {
                // A Pass reacted on meeting, or CanHit refused, and they have not parted since.
                touch.touching = true;
                return Go;
            }

            // The normal out of the second toward the first where they met; for two that began inside each other, at the start.
            var inside = t0 < 0.0f;
            var t = inside ? 0.0f : t0;
            var hx = px + t * dx;
            var hy = py + t * dy;
            var hz = pz + t * dz;
            var length = Trig.Sqrt(hx * hx + hy * hy + hz * hz);
            float nx;
            float ny;
            float nz;
            if (length > Tiny)
            {
                nx = hx / length;
                ny = hy / length;
                nz = hz / length;
            }
            else
            {
                // One middle on the other: against the first's motion from the second, else up.
                var along = Trig.Sqrt(move);
                if (along > Tiny)
                {
                    nx = -dx / along;
                    ny = -dy / along;
                    nz = -dz / along;
                }
                else
                {
                    nx = 0.0f;
                    ny = 0.0f;
                    nz = 1.0f;
                }
            }

            // The velocities where they met: gravity has acted only for the part of the frame before. Only a move that was
            // a free fall took the step's gravity whole: one that met the ground or rolled along it this step has the
            // ground's velocity, and none is given back, as at Hit.
            var rest = Interactions.Current().Step * frame * (1.0f - t);
            var lateA = movedA && first.fell ? first.gravity * rest : 0.0f;
            var lateB = movedB && second.fell ? second.gravity * rest : 0.0f;
            var avx = first.vx;
            var avy = first.vy;
            var avz = first.vz + lateA;
            var bvx = second.vx;
            var bvy = second.vy;
            var bvz = second.vz + lateB;
            var into = (avx - bvx) * nx + (avy - bvy) * ny + (avz - bvz) * nz;
            if (inside && touch.touching && into >= 0.0f)
            {
                // They touched at the last check and are not closing: they are parting, as after a bounce.
                return Go;
            }

            touch.touching = true;
            touch.contacted = true;
            touch.lastTouch = now;
            // A missile whose MissileContact is None checks no missile: it is met by one that does, and reacts to nothing.
            var reactsA = first.missileContact != Contact.None;
            var reactsB = second.missileContact != Contact.None;
            if ((reactsA && !Asks(first, second)) || (reactsB && !Asks(second, first)))
            {
                touch.held = true;
                return Go;
            }

            // Each is put where they met for the reactions, at the velocity it met at.
            var grid = Interactions.Grid;
            var hitAX = aX0 + t * (aX1 - aX0);
            var hitAY = aY0 + t * (aY1 - aY0);
            var hitAZ = aZ0 + t * (aZ1 - aZ0);
            var hitBX = bX0 + t * (bX1 - bX0);
            var hitBY = bY0 + t * (bY1 - bY0);
            var hitBZ = bZ0 + t * (bZ1 - bZ0);
            grid.Move(entryA, hitAX, hitAY);
            first.z = hitAZ;
            first.vx = avx;
            first.vy = avy;
            first.vz = avz;
            grid.Move(entryB, hitBX, hitBY);
            second.z = hitBZ;
            second.vx = bvx;
            second.vy = bvy;
            second.vz = bvz;
            var pointX = hitAX - radiusA * nx;
            var pointY = hitAY - radiusA * ny;
            var pointZ = hitAZ - radiusA * nz;
            var speed = into < 0.0f ? -into : 0.0f;
            var impactA = new Impact(ImpactKind.Missile, second, pointX, pointY, pointZ, nx, ny, nz, speed, first.missileContact, first.impactDamage);
            var impactB = new Impact(ImpactKind.Missile, first, pointX, pointY, pointZ, -nx, -ny, -nz, speed, second.missileContact, second.impactDamage);
            if (reactsA)
            {
                Flight.Impacted(first, impactA);
            }

            if (reactsB && !Ended(second))
            {
                Flight.Impacted(second, impactB);
            }

            // The outcomes, from the velocities the reactions left. A bounce keeps the two bouncinesses multiplied and
            // square-rooted, and the change is shared by mass: the heavier changes less; one with no mass against one with
            // mass takes all of it; two with none share it evenly. A None side is met as something that does not move: the
            // other takes all of it.
            into = (first.vx - second.vx) * nx + (first.vy - second.vy) * ny + (first.vz - second.vz) * nz;
            var both = first.bounciness * second.bounciness;
            var keep = both > 0.0f ? Trig.Sqrt(both) : 0.0f;
            var massA = first.mass;
            var massB = second.mass;
            float shareA;
            if (!reactsB)
            {
                shareA = 1.0f;
            }
            else if (!reactsA)
            {
                shareA = 0.0f;
            }
            else if (massA > 0.0f && massB > 0.0f)
            {
                shareA = massB / (massA + massB);
            }
            else if (massA > 0.0f)
            {
                shareA = 0.0f;
            }
            else if (massB > 0.0f)
            {
                shareA = 1.0f;
            }
            else
            {
                shareA = 0.5f;
            }

            var change = into < 0.0f ? -(1.0f + keep) * into : 0.0f;
            var passA = Take(first, entryA, impactA.contact, hitAX, hitAY, hitAZ, pointX, pointY, pointZ, nx, ny, nz, change * shareA, lateA, aX1, aY1, aZ1);
            var passB = Take(second, entryB, impactB.contact, hitBX, hitBY, hitBZ, pointX, pointY, pointZ, -nx, -ny, -nz, change * (1.0f - shareA), lateB, bX1, bY1, bZ1);
            if ((passA && reactsA) || (passB && reactsB))
            {
                // Neither reacts to the other again until they have parted. A None side flies on, but that is not a Pass:
                // a Bounce side meets it again while they touch, as it would a thing that does not move.
                touch.passed = true;
            }

            return Go;
        }

        /// <summary>
        /// One of two missiles takes its own outcome where they met: Stop ends it at the point of contact; Pass and None let it
        /// fly on from where its move took it (true); Bounce, and Slide, add its share of the change along the normal out of the
        /// other, and it stays where they met, the pairs waiting on it checking again if it is faster. Nothing for one that has
        /// begun to end; a reaction that moved it leaves it where it was put.
        /// </summary>
        private static bool Take(Missile missile, Entry entry, Contact outcome, float hitX, float hitY, float hitZ, float pointX, float pointY, float pointZ, float nx, float ny, float nz, float change, float late, float endX, float endY, float endZ)
        {
            if (Ended(missile))
            {
                return false;
            }

            var moved = entry.X != hitX || entry.Y != hitY || missile.z != hitZ;
            if (outcome == Contact.Stop)
            {
                StopAt(missile, entry, pointX, pointY, pointZ);
                return false;
            }

            if (outcome == Contact.Pass || outcome == Contact.None)
            {
                FlyOn(missile, entry, late, moved, endX, endY, endZ);
                return true;
            }

            var vx = missile.vx;
            var vy = missile.vy;
            var vz = missile.vz;
            missile.vx = vx + change * nx;
            missile.vy = vy + change * ny;
            missile.vz = vz + change * nz;
            Quicker(missile, vx * vx + vy * vy + vz * vz);
            Stay(missile, entry, moved, hitX, hitY, hitZ);
            return false;
        }

        /// <summary>Two missiles apart along their moves: not touching, and the pair waits until they could touch.</summary>
        private static int Parted(Missile first, Missile second, Touch touch, float ox, float oy, float reach, out float wait)
        {
            Leave(touch);
            wait = Closing(first, second, ox, oy, reach);
            return wait > 0.0f ? Later : Go;
        }

        /// <summary>The earliest seconds from now two missiles (ox, oy) apart, flat, could touch: from both speeds, and how fast each may speed up.</summary>
        private static float Closing(Missile first, Missile second, float ox, float oy, float reach)
        {
            var vx = second.vx;
            var vy = second.vy;
            var vz = second.vz;
            var gravity = second.gravity;
            var quicker = (gravity < 0.0f ? -gravity : gravity) + (second.acceleration > 0.0f ? second.acceleration : 0.0f);
            return Until(first, ox, oy, reach, Trig.Sqrt(vx * vx + vy * vy + vz * vz), quicker);
        }

        /// <summary>
        /// A missile's walk down the walls, right after its move in the pass: its ball is swept along its move against each
        /// wall's upright strip, as tall as the wall above the ground (or endless), a wall whose box does not hold the move
        /// passed over in a few compares, before anything kept of it is looked up. The wall met first along the move is met;
        /// after a Pass, a slow return to a wall's side that goes on along it, or nothing, the walk goes on for the walls
        /// further along, each met at most once in it; after a Stop or a bounce, which leave the ball where it met the wall,
        /// it ends. A wall the ball has left is forgotten.
        /// </summary>
        internal static void AgainstWalls(Missile missile)
        {
            var list = Walls.Walk();
            if (list == null)
            {
                return;
            }

            Moved(missile);
            var touches = missile.wallTouches;
            if (touches != null && touches.Count > 0)
            {
                Unwalled(touches);
            }

            wallRound++;
            var round = wallRound;
            var count = list.Count;
            for (var turn = 0; turn < count && !Ended(missile) && missile.wallContact != Contact.None; turn++)
            {
                var best = -1;
                var bestT = Always;
                var bestInside = false;
                var bestX = 0.0f;
                var bestY = 0.0f;
                var bestZ = 0.0f;
                var bestTop = 0.0f;
                for (var i = 0; i < count; i++)
                {
                    var wall = Raw.At(list, i);
                    if (wall.ending)
                    {
                        continue;
                    }

                    float enter;
                    bool inside;
                    float nx;
                    float ny;
                    float nz;
                    float top;
                    var found = Sweep(missile, wall, out enter, out inside, out nx, out ny, out nz, out top);
                    if (found == Far)
                    {
                        // Nothing kept to forget: a wall the ball was at last step is in its move's box, its start being
                        // there, and a missile moved from outside forgets its walls.
                        continue;
                    }

                    var touch = TouchOf(missile, wall);
                    if (found == Off)
                    {
                        if (touch != null && Beyond(missile, wall))
                        {
                            Forget(missile, wall);
                        }

                        continue;
                    }

                    if (touch != null && (touch.round == round || touch.passed || touch.held))
                    {
                        // Met in this walk already, or a Pass or a refusal not left since.
                        touch.touching = true;
                        continue;
                    }

                    if (enter < bestT)
                    {
                        best = i;
                        bestT = enter;
                        bestInside = inside;
                        bestX = nx;
                        bestY = ny;
                        bestZ = nz;
                        bestTop = top;
                    }
                }

                if (best < 0)
                {
                    break;
                }

                var met = Raw.At(list, best);
                var kept = TouchOf(missile, met);
                if (kept == null)
                {
                    kept = new Touch();
                    kept.wall = met;
                    if (missile.wallTouches == null)
                    {
                        missile.wallTouches = new List<Touch>();
                    }

                    Raw.Append(missile.wallTouches, kept);
                }

                kept.round = round;
                var next = Hit(missile, kept, ImpactKind.Wall, met, null, bestInside ? 0.0f : bestT, bestInside, bestX, bestY, bestZ, bestTop, 0.0f, 0.0f, missile.wallContact, met.bounciness);
                if (missile.wallTouches!.Count == 0)
                {
                    // A reaction moved it and it forgot its walls: the walk ends, and its next sweep starts where it was put.
                    break;
                }

                if (next == Hold)
                {
                    kept.held = true;
                }
                else if (next != Through)
                {
                    break;
                }
            }

            Walls.Done();
        }

        /// <summary>
        /// Sweeps a missile's ball along its move this step against a wall: a strip from end to end, the ball's radius either
        /// side and round at the ends (flat), from below the ground to the wall's height above the ground under it plus the
        /// radius (or endless). Far when the wall's box does not hold the move, Off when they do not meet, Met when they meet
        /// within the move, with the share of the move they met at (below 0 when it began inside), the normal out of the
        /// wall there (for one that began inside, out of the nearest face) and the top.
        /// </summary>
        private static int Sweep(Missile missile, Wall wall, out float enter, out bool inside, out float nx, out float ny, out float nz, out float top)
        {
            enter = 0.0f;
            inside = false;
            nx = 0.0f;
            ny = 0.0f;
            nz = 0.0f;
            top = Always;
            var entry = missile.entry!;
            var radius = missile.radius;
            var ax = missile.lastX;
            var ay = missile.lastY;
            var az = missile.lastZ;
            var bx = entry.X;
            var by = entry.Y;
            var bz = missile.z;
            float x1;
            float y1;
            float x2;
            float y2;
            Ends(wall, out x1, out y1, out x2, out y2);
            var near = radius + Skin;
            var lowX = (ax < bx ? ax : bx) - near;
            var highX = (ax < bx ? bx : ax) + near;
            var lowY = (ay < by ? ay : by) - near;
            var highY = (ay < by ? by : ay) + near;
            if ((x1 < lowX && x2 < lowX) || (x1 > highX && x2 > highX) || (y1 < lowY && y2 < lowY) || (y1 > highY && y2 > highY))
            {
                return Far;
            }

            var dx = bx - ax;
            var dy = by - ay;
            var dz = bz - az;
            var ex = x2 - x1;
            var ey = y2 - y1;
            var length = Trig.Sqrt(ex * ex + ey * ey);

            // The flat part: when along the move the ball's middle is within its radius of the wall, as the union of the
            // strip's straight part and its two round ends.
            var t0 = Always;
            var t1 = -Always;
            if (length > Tiny)
            {
                var ux = ex / length;
                var uy = ey / length;
                float s0;
                float s1;
                if (Slab(-uy * (ax - x1) + ux * (ay - y1), -uy * dx + ux * dy, -radius, radius, out s0, out s1))
                {
                    float a0;
                    float a1;
                    if (Slab(ux * (ax - x1) + uy * (ay - y1), ux * dx + uy * dy, 0.0f, length, out a0, out a1))
                    {
                        var from = s0 > a0 ? s0 : a0;
                        var to = s1 < a1 ? s1 : a1;
                        if (from <= to)
                        {
                            t0 = from;
                            t1 = to;
                        }
                    }
                }
            }

            End(ax - x1, ay - y1, dx, dy, radius, ref t0, ref t1);
            End(ax - x2, ay - y2, dx, dy, radius, ref t0, ref t1);
            if (t0 > t1 || t1 < 0.0f || t0 > 1.0f)
            {
                return Off;
            }

            // The height, read only now: the ground under the nearest point of the wall to where the ball is along the move.
            var z0 = -Always;
            var z1 = Always;
            var height = wall.height;
            if (height > 0.0f)
            {
                var at = t0 > 0.0f ? (t0 < 1.0f ? t0 : 1.0f) : 0.0f;
                float qx;
                float qy;
                Nearest(ax + at * dx, ay + at * dy, x1, y1, ex, ey, out qx, out qy);
                top = Ground.Height(qx, qy) + height + radius;
                if (dz == 0.0f)
                {
                    if (az > top)
                    {
                        return Off;
                    }
                }
                else if (dz < 0.0f)
                {
                    z0 = (top - az) / dz;
                }
                else
                {
                    z1 = (top - az) / dz;
                }
            }

            var first = t0 > z0 ? t0 : z0;
            var last = t1 < z1 ? t1 : z1;
            if (first > last || last < 0.0f || first > 1.0f)
            {
                return Off;
            }

            enter = first;
            inside = first < 0.0f;
            var t = inside ? 0.0f : first;
            var cx = ax + t * dx;
            var cy = ay + t * dy;
            float px;
            float py;
            Nearest(cx, cy, x1, y1, ex, ey, out px, out py);
            var ox = cx - px;
            var oy = cy - py;
            var off = Trig.Sqrt(ox * ox + oy * oy);
            if (height > 0.0f && (inside ? top - az <= radius - off : t0 < z0))
            {
                // Onto its top: from above, or, for a ball that began inside, nearer the top than the face.
                nz = 1.0f;
                return Met;
            }

            if (off > Tiny)
            {
                nx = ox / off;
                ny = oy / off;
                return Met;
            }

            // Its middle on the wall's line: out of the side it is moving from, else the wall's left.
            var lx = length > Tiny ? -ey / length : 1.0f;
            var ly = length > Tiny ? ex / length : 0.0f;
            if (lx * dx + ly * dy > 0.0f)
            {
                lx = -lx;
                ly = -ly;
            }

            nx = lx;
            ny = ly;
            return Met;
        }

        /// <summary>A wall's two ends where it is now: as made, moved by how far its place has moved since.</summary>
        private static void Ends(Wall wall, out float x1, out float y1, out float x2, out float y2)
        {
            var place = wall.entry!;
            var shiftX = place.X - wall.madeX;
            var shiftY = place.Y - wall.madeY;
            x1 = wall.x1 + shiftX;
            y1 = wall.y1 + shiftY;
            x2 = wall.x2 + shiftX;
            y2 = wall.y2 + shiftY;
        }

        /// <summary>When along a move a value starting at start and changing by change is between low and high: the shares of the move, from and to; false for never. A value that does not change is between them always or never.</summary>
        private static bool Slab(float start, float change, float low, float high, out float from, out float to)
        {
            if (change == 0.0f)
            {
                from = -Always;
                to = Always;
                return start >= low && start <= high;
            }

            var a = (low - start) / change;
            var b = (high - start) / change;
            if (a > b)
            {
                var swap = a;
                a = b;
                b = swap;
            }

            from = a;
            to = b;
            return true;
        }

        /// <summary>Widens the shares of a move a ball is within its radius of a wall by those it is within it of one of its ends, at (ox, oy) from the ball's start.</summary>
        private static void End(float ox, float oy, float dx, float dy, float radius, ref float t0, ref float t1)
        {
            var flat = dx * dx + dy * dy;
            var gap = ox * ox + oy * oy - radius * radius;
            float from;
            float to;
            if (flat <= 0.0f)
            {
                if (gap > 0.0f)
                {
                    return;
                }

                from = -Always;
                to = Always;
            }
            else
            {
                var half = ox * dx + oy * dy;
                var square = half * half - flat * gap;
                if (square < 0.0f)
                {
                    return;
                }

                var root = Trig.Sqrt(square);
                from = (-half - root) / flat;
                to = (-half + root) / flat;
            }

            if (from < t0)
            {
                t0 = from;
            }

            if (to > t1)
            {
                t1 = to;
            }
        }

        /// <summary>
        /// Puts a ball's middle at (x, y) back out at a wall's side, its radius from the wall: when it is nearer on its own
        /// side (the side of the normal (nx, ny)), away from the nearest point of the wall; and whenever it is on the wall's
        /// line or past it, whatever its radius, along the normal, its radius out and at least Clear, so a ball of radius 0
        /// is on its own side too.
        /// </summary>
        private static void Beside(Wall wall, float radius, float nx, float ny, ref float x, ref float y)
        {
            float x1;
            float y1;
            float x2;
            float y2;
            Ends(wall, out x1, out y1, out x2, out y2);
            var ex = x2 - x1;
            var ey = y2 - y1;
            float qx;
            float qy;
            Nearest(x, y, x1, y1, ex, ey, out qx, out qy);
            var ox = x - qx;
            var oy = y - qy;
            var off = Trig.Sqrt(ox * ox + oy * oy);
            var own = ox * nx + oy * ny > 0.0f;
            if (own && off >= radius)
            {
                return;
            }

            if (own && off > Tiny)
            {
                x = qx + ox / off * radius;
                y = qy + oy / off * radius;
                return;
            }

            var back = radius > Clear ? radius : Clear;
            x = qx + nx * back;
            y = qy + ny * back;
        }

        /// <summary>The nearest point of a wall from (x1, y1) along (ex, ey) to a point. It divides by the wall's length squared as it is, not by a square root squared: the game's square root, off by up to 2 parts in 100,000, put the point off along the wall by as much, which tilted a bounce's normal.</summary>
        private static void Nearest(float x, float y, float x1, float y1, float ex, float ey, out float qx, out float qy)
        {
            var square = ex * ex + ey * ey;
            var along = square > Tiny * Tiny ? ((x - x1) * ex + (y - y1) * ey) / square : 0.0f;
            if (along < 0.0f)
            {
                along = 0.0f;
            }
            else if (along > 1.0f)
            {
                along = 1.0f;
            }

            qx = x1 + along * ex;
            qy = y1 + along * ey;
        }

        /// <summary>Whether a ball apart from a wall along its move ended it farther than its radius and the Skin from the wall, so what it kept of the wall is forgotten.</summary>
        private static bool Beyond(Missile missile, Wall wall)
        {
            float x1;
            float y1;
            float x2;
            float y2;
            Ends(wall, out x1, out y1, out x2, out y2);
            var ex = x2 - x1;
            var ey = y2 - y1;
            var entry = missile.entry!;
            var bx = entry.X;
            var by = entry.Y;
            float qx;
            float qy;
            Nearest(bx, by, x1, y1, ex, ey, out qx, out qy);
            var ox = bx - qx;
            var oy = by - qy;
            var near = missile.radius + Skin;
            return ox * ox + oy * oy > near * near;
        }

        /// <summary>What a missile keeps of a wall its ball is at; null for none.</summary>
        private static Touch? TouchOf(Missile missile, Wall wall)
        {
            var touches = missile.wallTouches;
            if (touches == null)
            {
                return null;
            }

            var count = touches.Count;
            for (var i = 0; i < count; i++)
            {
                var touch = Raw.At(touches, i);
                if (touch.wall == wall)
                {
                    return touch;
                }
            }

            return null;
        }

        /// <summary>Forgets a wall a missile's ball has left, keeping the order of the rest: a Pass or a refusal may meet it again when it next comes to it.</summary>
        private static void Forget(Missile missile, Wall wall)
        {
            var touch = TouchOf(missile, wall);
            if (touch != null)
            {
                Remove(missile.wallTouches!, touch);
            }
        }

        /// <summary>Forgets the walls that have ended, keeping the order of the rest.</summary>
        private static void Unwalled(List<Touch> touches)
        {
            var kept = 0;
            var count = touches.Count;
            for (var i = 0; i < count; i++)
            {
                var touch = Raw.At(touches, i);
                if (!touch.wall!.ending)
                {
                    Raw.Put(touches, kept, touch);
                    kept++;
                }
            }

            Raw.Truncate(touches, kept);
        }

        /// <summary>Whether a missile has begun to end.</summary>
        private static bool Ended(Missile missile)
        {
            return missile.ending || missile.HasEnded;
        }
    }

    /// <summary>What the package keeps of a pair of a missile and a thing it checks: whether the ball touched the thing at the last check, whether a Pass is inside it, when it last met it, the thing's size, and, for a unit, its place at the last check for its velocity. Made when they meet; the pair's Data, or for a wall, kept in the missile's list while the ball is at it. Two missiles keep one, which both sides' pairs share and both missiles list, until both pairs have parted.</summary>
    internal sealed class Touch
    {
        /// <summary>Whether the ball touched the thing at the last check.</summary>
        public bool touching;

        /// <summary>Whether a Pass has reacted to the thing and the ball has not left it since.</summary>
        public bool passed;

        /// <summary>Whether the ball has met the thing since they met.</summary>
        public bool contacted;

        /// <summary>The step of the cycle the ball last met the thing or settled on its top at.</summary>
        public int lastTouch;

        /// <summary>Whether a unit's place has been seen at a check.</summary>
        public bool seen;

        /// <summary>The step of the cycle the unit's place was last seen at.</summary>
        public int step;

        /// <summary>The unit's place last seen, left to right.</summary>
        public float x;

        /// <summary>The unit's place last seen, bottom to top.</summary>
        public float y;

        /// <summary>The thing's radius.</summary>
        public float radius;

        /// <summary>The thing's height; a unit's is read at each contact, as its Body may change it.</summary>
        public float height;

        /// <summary>For a wall: the wall; the touch is in the missile's own list, not a pair's Data.</summary>
        public Wall? wall;

        /// <summary>For a wall: whether CanHit refused it and the ball has not left it since.</summary>
        public bool held;

        /// <summary>For a wall: the walk down the walls that last met it, so it is met at most once a walk.</summary>
        public int round;

        /// <summary>For two missiles: the one made first.</summary>
        public Missile? early;

        /// <summary>For two missiles: the one made after.</summary>
        public Missile? late;

        /// <summary>For two missiles: the earlier one's pair with the later, while it has one; an Interaction&lt;Missile, Missile&gt;.Pair kept as an Owner (see Pairing).</summary>
        public Owner? earlyPair;

        /// <summary>For two missiles: the later one's pair with the earlier, while it has one.</summary>
        public Owner? latePair;
    }

    /// <summary>A missile's checks against units: the package starts it on a missile whose UnitContact is not None.</summary>
    internal sealed class UnitContacts : Interaction<Missile, Unit>
    {
        /// <summary>Makes the checks; every unit is accepted, so a pair is not asked again before each tick (a guarded call per pair, every step).</summary>
        public UnitContacts()
        {
            RecheckAccepts = false;
        }

        /// <summary>Keeps what the pair needs: the unit's radius.</summary>
        public override void OnMeet(Pair pair)
        {
            pair.Data = Contacts.MetUnit(pair.Other);
        }

        /// <summary>Sweeps the missile's move against the unit; pauses or waits the pair as the check says.</summary>
        public override void OnTick(Pair pair)
        {
            float wait;
            var next = Contacts.UnitTick(pair.Holder, pair.Other, (Touch)pair.Data!, pair.OtherX, pair.OtherY, out wait);
            if (next == Contacts.Hold)
            {
                pair.Pause();
            }
            else if (next == Contacts.Later)
            {
                pair.Wait(wait);
            }
        }
    }

    /// <summary>A missile's checks against destructables: the package starts it on a missile whose DestructableContact is not None. A placed bridge is not one of them: its deck is its shape, met as the ground.</summary>
    internal sealed class DestructableContacts : Interaction<Missile, Destructable>
    {
        /// <summary>Makes the checks; whether a destructable is a placed bridge does not change, so a pair is not asked again before each tick (a guarded call per pair, every step).</summary>
        public DestructableContacts()
        {
            RecheckAccepts = false;
        }

        /// <summary>Refuses a placed bridge, which is ground where its deck is, not an upright cylinder.</summary>
        public override bool Accepts(Destructable other)
        {
            return Decks.count == 0 || !Decks.IsBridge(other);
        }

        /// <summary>Keeps what the pair needs: the destructable's size.</summary>
        public override void OnMeet(Pair pair)
        {
            pair.Data = Contacts.MetDestructable(pair.Other);
        }

        /// <summary>Sweeps the missile's move against the destructable; pauses or waits the pair as the check says.</summary>
        public override void OnTick(Pair pair)
        {
            float wait;
            var next = Contacts.DestructableTick(pair.Holder, pair.Other, (Touch)pair.Data!, pair.OtherX, pair.OtherY, out wait);
            if (next == Contacts.Hold)
            {
                pair.Pause();
            }
            else if (next == Contacts.Later)
            {
                pair.Wait(wait);
            }
        }
    }

    /// <summary>A missile's checks against items on the ground: the package starts it on a missile whose ItemContact is not None.</summary>
    internal sealed class ItemContacts : Interaction<Missile, Item>
    {
        /// <summary>Makes the checks; every item is accepted, so a pair is not asked again before each tick (a guarded call per pair, every step).</summary>
        public ItemContacts()
        {
            RecheckAccepts = false;
        }

        /// <summary>Keeps what the pair needs: the item's size.</summary>
        public override void OnMeet(Pair pair)
        {
            pair.Data = Contacts.MetItem();
        }

        /// <summary>Sweeps the missile's move against the item; pauses or waits the pair as the check says.</summary>
        public override void OnTick(Pair pair)
        {
            float wait;
            var next = Contacts.ItemTick(pair.Holder, pair.Other, (Touch)pair.Data!, pair.OtherX, pair.OtherY, out wait);
            if (next == Contacts.Hold)
            {
                pair.Pause();
            }
            else if (next == Contacts.Later)
            {
                pair.Wait(wait);
            }
        }
    }

    /// <summary>A missile's checks against other missiles: the package starts it on a missile whose MissileContact is not None. Of two missiles, the one made first checks the pair.</summary>
    internal sealed class MissileContacts : Interaction<Missile, Missile>
    {
        /// <summary>Makes the checks; every missile is accepted, so a pair is not asked again before each tick (a guarded call per pair, every step).</summary>
        public MissileContacts()
        {
            RecheckAccepts = false;
        }

        /// <summary>Keeps what the pair needs: the one state the two missiles keep of each other, shared with the other side's pair.</summary>
        public override void OnMeet(Pair pair)
        {
            pair.Data = Contacts.MetMissile(pair.Holder, pair.Other, pair);
        }

        /// <summary>Lets go of the shared state; once neither side holds it, it is dropped.</summary>
        public override void OnPart(Pair pair)
        {
            var touch = pair.Data as Touch;
            if (touch != null)
            {
                Contacts.PartedMissile(pair.Holder, touch);
            }
        }

        /// <summary>Sweeps the two missiles' moves against each other, through the one made first; pauses or waits the pair as the check says.</summary>
        public override void OnTick(Pair pair)
        {
            float wait;
            var next = Contacts.MissileTick(pair.Holder, pair.Other, (Touch)pair.Data!, out wait);
            if (next == Contacts.Hold)
            {
                pair.Pause();
            }
            else if (next == Contacts.Later)
            {
                pair.Wait(wait);
            }
        }
    }
}
