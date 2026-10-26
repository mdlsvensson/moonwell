using System;
using System.Collections.Generic;

namespace Moonwell.Engine
{
    /// <summary>
    /// A missile's contacts with units, destructables and items. At Create the package starts one interaction on the missile
    /// for each of those kinds whose setting is not None, belonging to the missile. Its range covers the missile's ball, the
    /// largest thing of the kind met so far and the missile's move this step. Each check of a pair sweeps the ball along the
    /// step's move, from where it began to where it is, against the other's upright cylinder where the other is now: the
    /// flat circle first, the heights only when that meets, so a fast missile cannot skip a footman between two steps. Apart,
    /// the pair waits for the earliest time they could touch, from their distance and the missile's speed (and a unit's
    /// fastest walk). At a contact, in order: CanHit (refused, nothing, and the pair is not checked again until it parts);
    /// OnImpact on the missile and its components; for a unit, OnStruck on its class, components and buffs; the damage;
    /// the outcome. The step's checks make no table; an Impact is made at a contact only.
    /// </summary>
    internal static class Contacts
    {
        /// <summary>The pair goes on: it is checked again at its next tick.</summary>
        internal const int Go = 0;

        /// <summary>The pair is paused until it parts: it was refused, or it reacted as a Pass.</summary>
        internal const int Hold = 1;

        /// <summary>The pair waits for the seconds given before its next check.</summary>
        internal const int Later = 2;

        /// <summary>The largest radius of a unit taken before a larger one is met.</summary>
        private const float FirstUnit = 64.0f;

        /// <summary>The fastest a unit walks, in game units a second: what a unit may add to the speed a missile closes on it at.</summary>
        private const float Walk = 522.0f;

        /// <summary>The bit of OnStruck in a class's mask.</summary>
        private const int StruckBit = 1;

        /// <summary>A length shorter than this gives no direction, for a normal.</summary>
        private const float Tiny = 0.0001f;

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

        /// <summary>The reactions each class writes, keyed by the class (never by an actor); made on first use.</summary>
        private static Dictionary<object, int>? masks;

        /// <summary>Forgets the largest radii met and the classes' masks; an install starts afresh.</summary>
        internal static void Reset()
        {
            largestUnit = FirstUnit;
            largestDestructable = Shapes.DestructableRadius;
            largestItem = Shapes.ItemRadius;
            masks = null;
        }

        /// <summary>Starts a new missile's checks: one interaction for each kind whose setting is not None.</summary>
        internal static void Begin(Missile missile)
        {
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

        /// <summary>A setting changed: a created missile that has not begun to end starts the checks of a kind set to other than None, if they have not started; the checks of a kind set to None pause, and those set back resume unless it rests.</summary>
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
        }

        /// <summary>Resumes a missile's checks of the kinds whose setting is not None, as it wakes from rest; the next comes at the next step.</summary>
        internal static void Resume(Missile missile)
        {
            Turn(missile.unitChecks, missile.unitContact, false);
            Turn(missile.destructableChecks, missile.destructableContact, false);
            Turn(missile.itemChecks, missile.itemContact, false);
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
            if (units == null && destructables == null && items == null)
            {
                return;
            }

            var dx = entry.X - missile.lastX;
            var dy = entry.Y - missile.lastY;
            var reach = missile.radius + Trig.Sqrt(dx * dx + dy * dy);
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
            else
            {
                if (missile.itemChecks != null)
                {
                    return;
                }

                checks = new ItemContacts();
                checks.range = missile.radius + largestItem;
                missile.itemChecks = checks;
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

            var entry = missile.entry!;
            var bx = entry.X;
            var by = entry.Y;
            var bz = missile.z;
            if (bx != missile.placedX || by != missile.placedY || bz != missile.placedZ)
            {
                // Moved from outside its flight since (its X, Y or Z set): it is not swept along the jump.
                missile.lastX = bx;
                missile.lastY = by;
                missile.lastZ = bz;
                Placed(missile, bx, by, bz);
            }

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
                    return Apart(missile, touch, bx - cx, by - cy, reach, closing, out wait);
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
                    return Apart(missile, touch, bx - cx, by - cy, reach, closing, out wait);
                }

                var root = Trig.Sqrt(square);
                t0 = (-half - root) / flat;
                t1 = (-half + root) / flat;
                if (t1 < 0.0f || t0 > 1.0f)
                {
                    return Apart(missile, touch, bx - cx, by - cy, reach, closing, out wait);
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

            // The velocity where it met the thing: gravity has acted only for the part of the step before.
            var vx = missile.vx;
            var vy = missile.vy;
            var late = flat > 0.0f || dz != 0.0f ? missile.gravity * Interactions.Current().Step * (1.0f - t) : 0.0f;
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
                return Go;
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

                grid.Move(entry, pointX, pointY);
                missile.z = pointZ;
                Flight.Place(missile, world, pointX, pointY, pointZ);
                Flight.Burst(missile, world, pointX, pointY, pointZ);
                missile.End();
                return Go;
            }

            if (outcome == Contact.Pass || outcome == Contact.None)
            {
                // It flies on from where its move took it, as it was; gravity acts on for the rest of the step. It does
                // not react to the thing again until it has left it.
                touch.passed = true;
                missile.vz = vz - late;
                if (moved)
                {
                    Jumped(missile, entry);
                    return Go;
                }

                grid.Move(entry, bx, by);
                missile.z = bz;
                Placed(missile, bx, by, bz);
                return Go;
            }

            // Bounce, and Slide, which is Bounce off anything but the ground and walls.
            if (into < 0.0f)
            {
                var keep = missile.bounciness;
                if (unit == null || !Shove(missile, unit, nx, ny, nz, into, keep))
                {
                    var back = (1.0f + keep) * into;
                    missile.vx = vx - back * nx;
                    missile.vy = vy - back * ny;
                    missile.vz = vz - back * nz;
                }

                if (Ended(missile))
                {
                    return Go;
                }
            }

            if (moved)
            {
                Jumped(missile, entry);
                return Go;
            }

            Placed(missile, hitX, hitY, hitZ);
            Flight.Place(missile, world, hitX, hitY, hitZ);
            return Go;
        }

        /// <summary>A reaction moved the missile at a contact: it stays where it was put, and its next sweep starts there.</summary>
        private static void Jumped(Missile missile, Entry entry)
        {
            var x = entry.X;
            var y = entry.Y;
            var z = missile.z;
            missile.lastX = x;
            missile.lastY = y;
            missile.lastZ = z;
            Placed(missile, x, y, z);
        }

        /// <summary>The ball is not touching the thing: a Pass that reacted to it may react again when it next enters.</summary>
        private static void Leave(Touch touch)
        {
            touch.touching = false;
            touch.passed = false;
        }

        /// <summary>The two are apart along the move: not touching, and the pair waits until they could touch, from their distance now and how fast they could close.</summary>
        private static int Apart(Missile missile, Touch touch, float ox, float oy, float reach, float closing, out float wait)
        {
            Leave(touch);
            wait = Until(missile, ox, oy, reach, closing);
            return wait > 0.0f ? Later : Go;
        }

        /// <summary>
        /// The earliest seconds from now a missile could touch a thing whose middle is (ox, oy) from it, flat: the gap between
        /// them, (distance - reach), over the missile's speed plus how fast the thing could close (a unit's fastest walk);
        /// with gravity or an Acceleration the missile may speed up, which the time allows for. 0 when they could touch at
        /// once, or never close.
        /// </summary>
        private static float Until(Missile missile, float ox, float oy, float reach, float closing)
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
            var faster = (gravity < 0.0f ? -gravity : gravity) + (missile.acceleration > 0.0f ? missile.acceleration : 0.0f);
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

        /// <summary>Whether a missile has begun to end.</summary>
        private static bool Ended(Missile missile)
        {
            return missile.ending || missile.HasEnded;
        }
    }

    /// <summary>What the package keeps of a pair of a missile and a thing it checks: whether the ball touched the thing at the last check, whether a Pass is inside it, when it last met it, the thing's size, and, for a unit, its place at the last check for its velocity. Made when they meet; the pair's Data.</summary>
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
    }

    /// <summary>A missile's checks against units: the package starts it on a missile whose UnitContact is not None.</summary>
    internal sealed class UnitContacts : Interaction<Missile, Unit>
    {
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

    /// <summary>A missile's checks against destructables: the package starts it on a missile whose DestructableContact is not None.</summary>
    internal sealed class DestructableContacts : Interaction<Missile, Destructable>
    {
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
}
