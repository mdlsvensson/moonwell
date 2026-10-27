using System;
using System.Collections.Generic;
using Moonwell.Hosting;

namespace Moonwell.Engine
{
    /// <summary>A missile's step, its launch from a unit, and its effects. The step works in floats and makes no table.</summary>
    internal static class Flight
    {
        /// <summary>A missile below this height ends.</summary>
        private const float Lowest = -2000.0f;

        /// <summary>A missile above this height ends.</summary>
        private const float Highest = 3000.0f;

        /// <summary>A mark nearer than this along the ground is at the launch's own place: the missile flies along the source's facing.</summary>
        private const float SamePlace = 1.0f;

        /// <summary>The share of a step a Duration or MaxDistance is forgiven, as a sum of steps can come out a little under in a float.</summary>
        private const float Forgiven = 0.001f;

        /// <summary>
        /// The least speed, in game units a second, at which the ground under a missile is taken to close on it between two
        /// reads (ALICE's slow case); a fast missile's is SlackShare times its speed along the ground plus its speed down.
        /// </summary>
        private const float Slack = 600.0f;

        /// <summary>How many times a missile's speed along the ground plus its speed down the ground is taken to close on it between two reads, when that is more than Slack (ALICE's form).</summary>
        private const float SlackShare = 3.0f;

        /// <summary>How near the ground, under its lowest point, a missile that begins a step counts as on it already.</summary>
        private const float OnGround = 0.5f;

        /// <summary>A Slide slower than this into the ground rolls instead of bouncing; a rolling missile moving out of the ground faster than this flies.</summary>
        private const float SlideSpeed = 5.0f;

        /// <summary>How far above the ground it would roll on, a rolling missile's free fall must be to leave the ground over a crest.</summary>
        private const float LiftOff = 0.5f;

        /// <summary>The step the pass is moving missiles by, for the shadows' count of time when the effects are placed.</summary>
        private static float stepping;

        /// <summary>
        /// Moves a missile one step: the speed change, gravity, its Homing's turn when one is due, then the move, added
        /// exactly for the step; a missile that follows the ground keeps its height above it. Then its Duration and
        /// MaxDistance, which end it after OnExpire, and the world's bounds, which end it. Then the ground, and its effects are
        /// placed and turned. A rolling missile rolls instead, and a resting one lies still until something gives it a velocity.
        /// </summary>
        public static void Step(Missile missile, float step, Grid grid, IPhysicsWorld world)
        {
            stepping = step;
            var start = missile.entry!;
            if (missile.keepLast)
            {
                missile.keepLast = false;
            }
            else
            {
                // Where the move begins: the contact checks later in the step sweep its ball from here.
                missile.lastX = start.X;
                missile.lastY = start.Y;
                missile.lastZ = missile.z;
                missile.lastStep = Tracking.StepNow;
            }

            if (missile.resting)
            {
                var place = start;
                if (place.X != missile.restX || place.Y != missile.restY || missile.z != missile.restZ || missile.groundGone)
                {
                    // Moved by its X, Y or Z, or the deck under it is gone: it flies from where it is, and the ground there is read again.
                    missile.rolling = false;
                    missile.groundRead = false;
                    missile.groundGone = false;
                }
                else if (missile.vx == 0.0f && missile.vy == 0.0f && missile.vz == 0.0f)
                {
                    missile.flown += step;
                    var lasts = missile.duration;
                    if (lasts > 0.0f && missile.flown >= lasts - step * Forgiven)
                    {
                        Expire(missile);
                    }

                    return;
                }

                // Its contact checks resume from the next step, so that check sweeps from here, over both steps.
                missile.resting = false;
                missile.keepLast = true;
                Contacts.Resume(missile);
            }

            var entry = start;
            var fromX = entry.X;
            var fromY = entry.Y;
            var fromZ = missile.z;
            if (missile.rolling && Roll(missile, step, grid, world, entry, fromX, fromY, fromZ))
            {
                return;
            }

            missile.fell = true;
            var vx = missile.vx;
            var vy = missile.vy;
            var vz = missile.vz;
            var acceleration = missile.acceleration;
            if (acceleration != 0.0f)
            {
                var speed = Trig.Sqrt(vx * vx + vy * vy + vz * vz);
                if (speed > 0.0f)
                {
                    var next = Faster(speed, acceleration * step, missile.maxSpeed);
                    var share = next / speed;
                    vx *= share;
                    vy *= share;
                    vz *= share;
                }
            }

            var gravity = missile.gravity;
            var rise = vz * step - gravity * step * step * 0.5f;
            vz -= gravity * step;

            missile.vx = vx;
            missile.vy = vy;
            missile.vz = vz;
            var homing = missile.homing;
            if (homing != null && homing.Turn(missile, step, fromX, fromY, fromZ))
            {
                if (missile.ending || missile.HasEnded)
                {
                    return;
                }

                // Turned: the move is worked out again from the new velocity, which is the one after this step's gravity.
                vx = missile.vx;
                vy = missile.vy;
                vz = missile.vz;
                rise = vz * step + gravity * step * step * 0.5f;
            }

            var x = fromX + vx * step;
            var y = fromY + vy * step;
            var z = fromZ + rise;
            if (missile.followsGround)
            {
                var below = missile.groundRead && missile.groundX == fromX && missile.groundY == fromY ? Ground.Last(missile) : Ground.Height(fromX, fromY, fromZ);
                var ground = Ground.Height(x, y, fromZ);
                z = ground + z - below;
                missile.groundRead = true;
                missile.groundPending = false;
                missile.groundX = x;
                missile.groundY = y;
                missile.groundZ = ground;
                missile.groundSince = 0.0f;
            }

            grid.Move(entry, x, y);
            missile.z = z;
            if (Ends(missile, world, fromX, fromY, fromZ, x, y, z, step))
            {
                return;
            }

            var contact = missile.groundContact;
            if (contact != Contact.None && contact != Contact.Pass && Land(missile, step, grid, world, entry, fromX, fromY, fromZ, x, y, z, rise))
            {
                return;
            }

            Place(missile, world, x, y, z);
        }

        /// <summary>
        /// Counts a missile's step and the distance it moved, and ends it when its Duration or MaxDistance has run out (after
        /// OnExpire) or it is outside the world's bounds, its effects drawn at its last place first. Whether it ended.
        /// </summary>
        private static bool Ends(Missile missile, IPhysicsWorld world, float fromX, float fromY, float fromZ, float x, float y, float z, float step)
        {
            var dx = x - fromX;
            var dy = y - fromY;
            var dz = z - fromZ;
            missile.flown += step;
            missile.travelled += Trig.Sqrt(dx * dx + dy * dy + dz * dz);
            var duration = missile.duration;
            var maxDistance = missile.maxDistance;
            var vx = missile.vx;
            var vy = missile.vy;
            var vz = missile.vz;
            if ((duration > 0.0f && missile.flown >= duration - step * Forgiven) || (maxDistance > 0.0f && missile.travelled >= maxDistance - Trig.Sqrt(vx * vx + vy * vy + vz * vz) * step * Forgiven))
            {
                Place(missile, world, x, y, z);
                Expire(missile);
                return true;
            }

            if (x < Pass.minX || x > Pass.maxX || y < Pass.minY || y > Pass.maxY || z < Lowest || z > Highest)
            {
                Place(missile, world, x, y, z);
                missile.End();
                return true;
            }

            return false;
        }

        /// <summary>
        /// The ground's contact after a flying missile's move, its ball's lowest point against the ground. The ground is read
        /// only when the missile may have reached it: once its height above the ground last read is no more than a slack
        /// times the seconds since, the slack the more of Slack and SlackShare times its speed along the ground plus its
        /// speed down. That is checked a step at a time from where it is, so a change of its velocity is seen at once; one
        /// that follows the ground was read this step.
        /// Below the ground, its move is cast against it for the point of contact. Whether it placed or ended the missile.
        /// With no deck living, a read where the missile is above its span's highest ground is put off (Ground.Last makes
        /// it): it could only find the ground below, which changes nothing but what is kept of the read, and the next
        /// step's check is made from the span's bounds while they decide it, as they do for whatever the read finds.
        /// </summary>
        private static bool Land(Missile missile, float step, Grid grid, IPhysicsWorld world, Entry entry, float fromX, float fromY, float fromZ, float x, float y, float z, float rise)
        {
            var radius = missile.radius;
            var bottom = z - radius;
            float hitX;
            float hitY;
            float hitZ;
            var fraction = 1.0f;
            // The height the decks were taken not above when the contact was found: the normal is read with it, so it is
            // the normal of the surface the contact is on. Read with the contact's own height, a deck the contact is on
            // could be left out by the last bit of a float, and the terrain beneath it would give the normal.
            var limit = fromZ;
            if (missile.followsGround)
            {
                // It keeps its height above the ground, so it meets the ground only when it comes down onto it.
                var ground = Ground.Last(missile);
                if (bottom > ground || rise >= 0.0f)
                {
                    return false;
                }

                hitX = x;
                hitY = y;
                hitZ = ground;
            }
            else
            {
                missile.groundSince += step;
                // Over a bridge the ground is read every step: a deck rises from the water as steeply as a cliff, and a
                // missile let fall past its top between two reads would be under it, where it is not met.
                if (missile.groundRead && (Decks.count == 0 || !Decks.Near(x, y)))
                {
                    var since = missile.groundSince;
                    if (missile.groundPending)
                    {
                        // The ground there is above spanLow and below spanHigh, and Far is no less for a lower ground: not Far
                        // with spanLow reads it for any ground there, and Far with spanHigh skips it for any. Between, the
                        // put-off read is made.
                        if (Far(missile, bottom - missile.spanLow, since) && (Far(missile, bottom - missile.spanHigh, since) || Far(missile, bottom - Ground.Last(missile), since)))
                        {
                            return false;
                        }
                    }
                    else if (Far(missile, bottom - missile.groundZ, since))
                    {
                        return false;
                    }
                }

                if (x >= missile.spanMinX && x <= missile.spanMaxX && y >= missile.spanMinY && y <= missile.spanMaxY && bottom > missile.spanHigh && Decks.count == 0 && missile.spanGrid == Terrain.Grid)
                {
                    // Above the highest ground of its span: the read could only find the ground below. It is put off.
                    Ground.reads++;
                    missile.groundRead = true;
                    missile.groundPending = true;
                    missile.groundX = x;
                    missile.groundY = y;
                    missile.groundSince = 0.0f;
                    missile.throughGround = false;
                    return false;
                }

                var ground = Ground.Height(x, y, fromZ);
                missile.groundRead = true;
                missile.groundPending = false;
                missile.groundX = x;
                missile.groundY = y;
                missile.groundZ = ground;
                missile.groundSince = 0.0f;
                if (bottom > ground)
                {
                    missile.throughGround = false;
                    if (Decks.count == 0 && (x < missile.spanMinX || x > missile.spanMaxX || y < missile.spanMinY || y > missile.spanMaxY || missile.spanGrid != Terrain.Grid))
                    {
                        // Out of its span: the span of where it is, for the reads after.
                        var terrain = Terrain.Grid;
                        missile.spanGrid = terrain;
                        terrain.Span(x, y, out missile.spanMinX, out missile.spanMinY, out missile.spanMaxX, out missile.spanMaxY, out missile.spanLow, out missile.spanHigh);
                    }

                    return false;
                }

                if (missile.throughGround)
                {
                    return false;
                }

                float startGap;
                if (!Ground.Cast(fromX, fromY, fromZ - radius, x, y, bottom, out hitX, out hitY, out hitZ, out fraction, out startGap))
                {
                    hitX = x;
                    hitY = y;
                    hitZ = ground;
                    fraction = 1.0f;
                }
                else if (startGap <= OnGround)
                {
                    // It began the step on the ground, near enough.
                    fraction = 0.0f;
                }
                else
                {
                    // The cast takes the decks not above the ball's lowest point at the start.
                    limit = fromZ - radius;
                }
            }

            // The velocity where it met the ground: gravity has acted only for the part of the step before.
            var vx = missile.vx;
            var vy = missile.vy;
            var vz = missile.vz + missile.gravity * step * (1.0f - fraction);
            var again = fraction <= 0.0f;
            if (again)
            {
                // It began the step on the ground: it left it and came back within the step, or went along it. It is met
                // at the end of its move, on the ground there, coming back as fast as it left; into a cliff's face, at the
                // start of its move.
                hitX = x;
                hitY = y;
                hitZ = Ground.Last(missile);
                limit = fromZ;
            }

            float nx;
            float ny;
            float nz;
            Ground.Normal(hitX, hitY, limit, out nx, out ny, out nz);
            if (nz == 0.0f && vx * nx + vy * ny >= 0.0f)
            {
                // A cliff's face it is not moving into, as one falling onto it: it meets the blend.
                Ground.Slope(hitX, hitY, limit, out nx, out ny, out nz);
            }
            else if (again && nz == 0.0f)
            {
                // Along the ground into a cliff's face: met at the face from where it was, as Roll meets it, not at the end
                // of its move up the cliff's blend.
                hitX = fromX;
                hitY = fromY;
                hitZ = fromZ - radius;
            }

            if (again && nz > 0.0f)
            {
                var leaving = vx * nx + vy * ny + vz * nz;
                if (leaving > 0.0f)
                {
                    vx -= 2.0f * leaving * nx;
                    vy -= 2.0f * leaving * ny;
                    vz -= 2.0f * leaving * nz;
                }

                if (missile.groundContact == Contact.Bounce)
                {
                    // A Bounce keeps the step's gravity along the ground, as the guide's loop keeps it when it slides on
                    // after a hit: without it, a Bounce on a slope meets it at the start of every step and stands where it
                    // is. A Slide that comes along the ground rolls, and Roll pulls it down the slope against its Friction.
                    var pull = missile.gravity * step;
                    vx += pull * nz * nx;
                    vy += pull * nz * ny;
                    vz -= pull * (1.0f - nz * nz);
                }
            }

            Meet(missile, step, grid, world, entry, hitX, hitY, hitZ, nx, ny, nz, vx, vy, vz, x, y, z, again);
            return true;
        }

        /// <summary>Whether a missile this high above the ground, so many seconds after its last read, cannot have reached it yet: the slack check of Land. True for no less a height once true.</summary>
        private static bool Far(Missile missile, float high, float since)
        {
            if (high > Slack * since)
            {
                // Its speed is worked out only when the least slack does not bring it to a read.
                var alongX = missile.vx;
                var alongY = missile.vy;
                var down = missile.vz < 0.0f ? -missile.vz : 0.0f;
                var fast = SlackShare * (Trig.Sqrt(alongX * alongX + alongY * alongY) + down);
                if (fast <= Slack || high > fast * since)
                {
                    return true;
                }
            }

            return false;
        }

        /// <summary>
        /// A missile meets the ground at a point (its ball's lowest point there), with the ground's normal and the velocity
        /// it met it at. Moving out of the ground, it is set on it and goes on. Else OnImpact runs, then the outcome: Stop ends
        /// it at the point, its effects moved there and its ImpactModel and ImpactSound played there; Bounce reflects it,
        /// keeping its Bounciness of the speed into the ground; Slide bounces, or rolls once slower than SlideSpeed into it;
        /// None and Pass let it on through to where its move took it (endX, endY, endZ). A Bounce or Slide that was on the
        /// ground already (<paramref name="along"/>) and comes back to it slower than SlideSpeed goes on along it with no
        /// impact, a Slide rolling, and a Bounce losing its Friction a second for the step, as a Slide rolling does: stopped
        /// where the slope's pull is no more than its friction, it rests and OnRest runs. A Stop that touches the ground
        /// stops, however slowly.
        /// </summary>
        private static void Meet(Missile missile, float step, Grid grid, IPhysicsWorld world, Entry entry, float hitX, float hitY, float hitZ, float nx, float ny, float nz, float vx, float vy, float vz, float endX, float endY, float endZ, bool along)
        {
            var radius = missile.radius;
            var into = vx * nx + vy * ny + vz * nz;
            missile.fell = false;
            grid.Move(entry, hitX, hitY);
            missile.z = hitZ + radius;
            missile.groundRead = false;
            var setting = missile.groundContact;
            if (setting != Contact.Stop && (into >= 0.0f || (along && -into < SlideSpeed)))
            {
                if (into < 0.0f)
                {
                    vx -= into * nx;
                    vy -= into * ny;
                    vz -= into * nz;
                }

                if (setting == Contact.Slide && nz > 0.0f)
                {
                    missile.rolling = true;
                }
                else if (along && nz > 0.0f && setting == Contact.Bounce)
                {
                    // A Bounce going along the ground loses its Friction, as Roll does a Slide.
                    var friction = missile.friction;
                    var speed = Trig.Sqrt(vx * vx + vy * vy + vz * vz);
                    var lost = friction * step;
                    if (speed <= lost)
                    {
                        missile.vx = 0.0f;
                        missile.vy = 0.0f;
                        missile.vz = 0.0f;
                        Place(missile, world, hitX, hitY, hitZ + radius);
                        var steep = 1.0f - nz * nz;
                        if ((steep > 0.0f ? missile.gravity * Trig.Sqrt(steep) : 0.0f) <= friction)
                        {
                            Rest(missile);
                        }

                        return;
                    }

                    var share = (speed - lost) / speed;
                    vx *= share;
                    vy *= share;
                    vz *= share;
                }

                missile.vx = vx;
                missile.vy = vy;
                missile.vz = vz;
                Place(missile, world, hitX, hitY, hitZ + radius);
                return;
            }

            // OnImpact reads the velocity at the point of contact, and the outcome works from the velocity it leaves.
            missile.vx = vx;
            missile.vy = vy;
            missile.vz = vz;
            var impact = new Impact(ImpactKind.Ground, null, hitX, hitY, hitZ, nx, ny, nz, into < 0.0f ? -into : 0.0f, setting, missile.impactDamage);
            Impacted(missile, impact);
            if (missile.ending || missile.HasEnded)
            {
                return;
            }

            vx = missile.vx;
            vy = missile.vy;
            vz = missile.vz;
            into = vx * nx + vy * ny + vz * nz;

            var outcome = impact.contact;
            if (outcome == Contact.Stop)
            {
                missile.z = hitZ;
                Place(missile, world, hitX, hitY, hitZ);
                Burst(missile, world, hitX, hitY, hitZ);
                missile.End();
                return;
            }

            if (outcome == Contact.None || outcome == Contact.Pass)
            {
                missile.throughGround = true;
                missile.rolling = false;
                grid.Move(entry, endX, endY);
                missile.z = endZ;
                Place(missile, world, endX, endY, endZ);
                return;
            }

            if (into >= 0.0f)
            {
                // The reaction left it moving out of the ground, or along it: it goes on so; a Slide along the ground rolls.
                if (outcome == Contact.Slide && nz > 0.0f && into < SlideSpeed)
                {
                    missile.rolling = true;
                }
            }
            else if (outcome == Contact.Slide && -into < SlideSpeed)
            {
                // Slow: it goes on along the surface, rolling on the ground, or along a cliff's face.
                vx -= into * nx;
                vy -= into * ny;
                vz -= into * nz;
                if (nz > 0.0f)
                {
                    missile.rolling = true;
                }
            }
            else
            {
                var back = (1.0f + missile.bounciness) * into;
                vx -= back * nx;
                vy -= back * ny;
                vz -= back * nz;
            }

            missile.vx = vx;
            missile.vy = vy;
            missile.vz = vz;
            Place(missile, world, hitX, hitY, hitZ + radius);
        }

        /// <summary>
        /// Rolls a missile one step along the ground: what of its velocity goes into the ground is taken away, the slope
        /// pulls it down it with Physics.Gravity, and it loses its Friction a second. It leaves the ground over a crest,
        /// where the ground falls away faster than it would fall, and meets a cliff's face as a wall. When it has stopped
        /// and the slope cannot move it against its friction, it rests and OnRest runs. False, and it flies this step
        /// instead, when it is no longer a Slide or something is moving it out of the ground.
        /// </summary>
        private static bool Roll(Missile missile, float step, Grid grid, IPhysicsWorld world, Entry entry, float fromX, float fromY, float fromZ)
        {
            if (missile.groundContact != Contact.Slide)
            {
                missile.rolling = false;
                return false;
            }

            var radius = missile.radius;
            float nx;
            float ny;
            float nz;
            Ground.Slope(fromX, fromY, fromZ, out nx, out ny, out nz);
            var vx = missile.vx;
            var vy = missile.vy;
            var vz = missile.vz;
            var into = vx * nx + vy * ny + vz * nz;
            if (into > SlideSpeed)
            {
                missile.rolling = false;
                missile.groundRead = false;
                return false;
            }

            missile.fell = false;
            vx -= into * nx;
            vy -= into * ny;
            vz -= into * nz;
            var gravity = Physics.Current().Gravity;
            vx += gravity * nz * nx * step;
            vy += gravity * nz * ny * step;
            vz += gravity * (nz * nz - 1.0f) * step;
            var steep = 1.0f - nz * nz;
            var pull = steep > 0.0f ? gravity * Trig.Sqrt(steep) : 0.0f;
            var friction = missile.friction;
            var speed = Trig.Sqrt(vx * vx + vy * vy + vz * vz);
            var lost = friction * step;
            var stopped = speed <= lost;
            var x = fromX;
            var y = fromY;
            var z = fromZ;
            if (stopped)
            {
                vx = 0.0f;
                vy = 0.0f;
                vz = 0.0f;
            }
            else
            {
                var share = (speed - lost) / speed;
                vx *= share;
                vy *= share;
                vz *= share;
                x = fromX + vx * step;
                y = fromY + vy * step;
                var bottom = fromZ - radius;
                var ground = Ground.Height(x, y, fromZ);
                if (ground > bottom)
                {
                    float wx;
                    float wy;
                    float wz;
                    Ground.Normal(x, y, fromZ, out wx, out wy, out wz);
                    if (wz == 0.0f && vx * wx + vy * wy < 0.0f)
                    {
                        // A cliff's face ahead: met as a wall, from where it is.
                        missile.vx = vx;
                        missile.vy = vy;
                        missile.vz = vz;
                        Meet(missile, step, grid, world, entry, fromX, fromY, bottom, wx, wy, wz, vx, vy, vz, x, y, ground + radius, true);
                        if (!missile.ending && !missile.HasEnded)
                        {
                            Ends(missile, world, fromX, fromY, fromZ, entry.X, entry.Y, missile.z, step);
                        }

                        return true;
                    }
                }

                var follow = ground + radius;
                var fall = missile.gravity;
                var free = fromZ + vz * step - fall * step * step * 0.5f;
                if (free > follow + LiftOff)
                {
                    // Over a crest: the ground falls away faster than it would fall, so it flies.
                    missile.fell = true;
                    z = free;
                    vz -= fall * step;
                    missile.rolling = false;
                    missile.groundRead = false;
                }
                else
                {
                    z = follow;
                }
            }

            missile.vx = vx;
            missile.vy = vy;
            missile.vz = vz;
            grid.Move(entry, x, y);
            missile.z = z;
            if (Ends(missile, world, fromX, fromY, fromZ, x, y, z, step))
            {
                return true;
            }

            if (stopped)
            {
                if (pull <= friction)
                {
                    Rest(missile);
                }

                return true;
            }

            Place(missile, world, x, y, z);
            return true;
        }

        /// <summary>Runs OnImpact on a missile, then on its components in the order they were added, each with the missile's owner as the current owner and each reporting an error it throws; none runs once the missile has ended.</summary>
        internal static void Impacted(Missile missile, Impact impact)
        {
            var previous = Owner.current;
            Owner.current = missile.Owner ?? Game.Instance;
            try
            {
                missile.OnImpact(impact);
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(missile) + ".OnImpact", error);
            }

            var count = missile.BeginDispatch();
            for (var i = 0; i < count && !missile.ending && !missile.HasEnded; i++)
            {
                var component = missile.ComponentAt(i) as MissileComponent;
                if (component == null || component.HasEnded)
                {
                    continue;
                }

                try
                {
                    component.OnImpact(impact);
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(component) + ".OnImpact", error);
                }
            }

            missile.EndDispatch();
            Owner.current = previous;
        }

        /// <summary>Lays a missile still on the ground, with no velocity, and runs OnRest on it, then on its components in the order they were added, each with the missile as the current owner and each reporting an error it throws; none runs once the missile has ended.</summary>
        private static void Rest(Missile missile)
        {
            missile.resting = true;
            missile.vx = 0.0f;
            missile.vy = 0.0f;
            missile.vz = 0.0f;
            missile.restX = missile.entry!.X;
            missile.restY = missile.entry.Y;
            missile.restZ = missile.z;
            missile.groundGone = false;
            Contacts.Pause(missile);
            var previous = Owner.current;
            Owner.current = missile;
            try
            {
                missile.OnRest();
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(missile) + ".OnRest", error);
            }

            var count = missile.BeginDispatch();
            for (var i = 0; i < count && !missile.ending && !missile.HasEnded; i++)
            {
                var component = missile.ComponentAt(i) as MissileComponent;
                if (component == null || component.HasEnded)
                {
                    continue;
                }

                try
                {
                    component.OnRest();
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(component) + ".OnRest", error);
                }
            }

            missile.EndDispatch();
            Owner.current = previous;
        }

        /// <summary>Plays a missile's ImpactModel once (an effect made and destroyed at once, which plays its death) and its ImpactSound at a point; nothing for an empty one.</summary>
        internal static void Burst(Missile missile, IPhysicsWorld world, float x, float y, float z)
        {
            var model = missile.impactModel;
            if (model.Length > 0)
            {
                var made = world.AddEffect(model, x, y);
                world.SetEffectPosition(made, x, y, z);
                world.RemoveEffect(made);
            }

            var sound = missile.impactSound;
            if (sound.Length > 0)
            {
                world.PlaySoundAt(sound, x, y, z);
            }
        }

        /// <summary>A speed after a change: down to 0 at the least, and up to the cap when there is one, unless it was over it already.</summary>
        private static float Faster(float speed, float change, float cap)
        {
            var next = speed + change;
            if (next < 0.0f)
            {
                return 0.0f;
            }

            if (change > 0.0f && cap > 0.0f && next > cap)
            {
                return speed > cap ? speed : cap;
            }

            return next;
        }

        /// <summary>Runs OnExpire with the missile as the current owner, reporting an error it throws, and ends the missile.</summary>
        private static void Expire(Missile missile)
        {
            var previous = Owner.current;
            Owner.current = missile;
            try
            {
                missile.OnExpire();
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(missile) + ".OnExpire", error);
            }

            Owner.current = previous;
            missile.End();
        }

        /// <summary>
        /// Launches a missile from a unit toward a mark and creates it. It starts at the unit's place, its LaunchOffset ahead
        /// along the unit's facing, and its LaunchHeight above the ground and the unit's fly height there. The mark is
        /// <paramref name="above"/> over the ground at (<paramref name="x"/>, <paramref name="y"/>); when
        /// <paramref name="level"/>, it is at the launch's own height above the ground, or with an Arc its radius above the
        /// ground, so its bottom lands on the ground there. The source and its player are the missile's.
        /// </summary>
        public static void Launch(Unit source, Missile missile, float x, float y, float above, bool level, Owner? owner)
        {
            Physics.EnsureInstalled();
            var facing = source.Facing;
            var sourceX = source.X;
            var sourceY = source.Y;
            var offset = missile.launchOffset;
            var startX = sourceX + offset * Angles.Cos(facing);
            var startY = sourceY + offset * Angles.Sin(facing);
            var startAbove = source.FlyHeight + missile.launchHeight;
            var startZ = Ground.Height(sourceX, sourceY) + startAbove;
            if (level)
            {
                // With an Arc its middle is aimed at the ground plus its radius, so its bottom lands on the mark.
                above = missile.arc > 0.0f ? missile.radius : startAbove;
            }

            var dz = missile.followsGround ? above - startAbove : Ground.Height(x, y) + above - startZ;
            Aim(missile, x - startX, y - startY, dz, facing);
            missile.source = source;
            Missile.Create(missile, startX, startY, startZ, source.Player, owner);
        }

        /// <summary>
        /// Sets a missile's velocity at its Speed toward a mark a way off. With an Arc it flies along the ground at its Speed
        /// and is given the Gravity and lift that peak at Arc times the distance above the line to the mark and land on it,
        /// as the game's Missile Arc (an Arc with no Speed fails in a development build); without one it flies straight at
        /// the mark. A mark at its own place sends it level along
        /// the facing, in degrees.
        /// </summary>
        private static void Aim(Missile missile, float dx, float dy, float dz, float facing)
        {
            var speed = missile.Speed;
            if (missile.arc > 0.0f && speed <= 0.0f)
            {
                Dev.Check(false, Names.NameOf(missile) + ".Fire: an Arc needs a Speed above 0, the speed it flies along the ground at");
            }
            var distance = Trig.Sqrt(dx * dx + dy * dy);
            if (distance < SamePlace)
            {
                missile.vx = speed * Angles.Cos(facing);
                missile.vy = speed * Angles.Sin(facing);
                missile.vz = 0.0f;
                return;
            }

            var arc = missile.arc;
            if (arc > 0.0f && speed > 0.0f)
            {
                var time = distance / speed;
                var peak = arc * distance;
                missile.gravity = 8.0f * peak / (time * time);
                missile.vx = dx / time;
                missile.vy = dy / time;
                missile.vz = dz / time + 4.0f * peak / time;
                return;
            }

            var share = speed / Trig.Sqrt(distance * distance + dz * dz);
            missile.vx = dx * share;
            missile.vy = dy * share;
            missile.vz = dz * share;
        }

        /// <summary>Makes a missile's effects at a place, at its height, as it is created: its model's, then those AddEffect listed.</summary>
        public static void Show(Missile missile, float x, float y)
        {
            missile.shown = true;
            var world = Physics.World;
            MakeLook(missile, world, x, y);
            var models = missile.extraModels;
            if (models != null)
            {
                for (var i = 0; i < models.Count; i++)
                {
                    ShowExtra(missile, i, x, y, world);
                }
            }
        }

        /// <summary>Makes the effect AddEffect listed at an index, where the missile is, after Create.</summary>
        public static void ShowExtra(Missile missile, int index)
        {
            ShowExtra(missile, index, missile.X, missile.Y, Physics.World);
        }

        /// <summary>Destroys a missile's model's effect and makes it again of its Model, where it is, with its scale, tint and facing.</summary>
        public static void Remake(Missile missile)
        {
            var world = Physics.World;
            var old = missile.look;
            missile.look = null;
            if (old != null)
            {
                world.RemoveEffect(old);
            }

            MakeLook(missile, world, missile.X, missile.Y);
        }

        /// <summary>Destroys a missile's effects; nothing is shown after.</summary>
        public static void Hide(Missile missile)
        {
            if (!missile.shown)
            {
                return;
            }

            missile.shown = false;
            var world = Physics.World;
            var look = missile.look;
            missile.look = null;
            if (look != null)
            {
                world.RemoveEffect(look);
            }

            var extras = missile.extras;
            if (extras != null)
            {
                for (var i = 0; i < extras.Count; i++)
                {
                    world.RemoveEffect(extras[i]);
                }

                extras.Clear();
            }
        }

        /// <summary>Gives an effect a missile's tint.</summary>
        public static void Colour(Missile missile, effect target)
        {
            var world = Physics.World;
            world.SetEffectColor(target, missile.tintRed, missile.tintGreen, missile.tintBlue);
            world.SetEffectAlpha(target, missile.tintAlpha);
        }

        /// <summary>Puts a missile's effects at its place, each at its height above it, and turns them to the velocity when it faces its flight; its shadows and sounds follow.</summary>
        internal static void Place(Missile missile, IPhysicsWorld world, float x, float y, float z)
        {
            var look = missile.look;
            if (look != null)
            {
                world.SetEffectPosition(look, x, y, z + missile.modelHeight);
            }

            var extras = missile.extras;
            if (extras != null)
            {
                var heights = missile.extraHeights!;
                var count = extras.Count;
                for (var i = 0; i < count; i++)
                {
                    world.SetEffectPosition(Raw.At(extras, i), x, y, z + Raw.At(heights, i));
                }
            }

            if (missile.facesFlight)
            {
                Face(missile, world);
            }

            if (missile.shadows != null)
            {
                Shadow.Follow(missile, world, x, y, z, stepping);
            }

            if (missile.sounds != null)
            {
                LoopingSound.Follow(missile, world, x, y, z);
            }
        }

        /// <summary>Turns a missile's effects to its velocity: yaw 0 east, pitch positive nose down, in radians. A still missile, or one whose velocity has not changed since, is left as it is.</summary>
        private static void Face(Missile missile, IPhysicsWorld world)
        {
            var vx = missile.vx;
            var vy = missile.vy;
            var vz = missile.vz;
            if ((vx == 0.0f && vy == 0.0f && vz == 0.0f) || (missile.faced && vx == missile.facedX && vy == missile.facedY && vz == missile.facedZ))
            {
                return;
            }

            var yaw = Trig.Atan2(vy, vx);
            var pitch = -Trig.Atan2(vz, Trig.Sqrt(vx * vx + vy * vy));
            missile.faced = true;
            missile.facedX = vx;
            missile.facedY = vy;
            missile.facedZ = vz;
            missile.facedYaw = yaw;
            missile.facedPitch = pitch;
            var look = missile.look;
            if (look != null)
            {
                world.SetEffectOrientation(look, yaw, pitch, 0.0f);
            }

            var extras = missile.extras;
            if (extras != null)
            {
                var count = extras.Count;
                for (var i = 0; i < count; i++)
                {
                    world.SetEffectOrientation(Raw.At(extras, i), yaw, pitch, 0.0f);
                }
            }
        }

        /// <summary>Makes a missile's model's effect at a place, at its model height, with its scale, tint and facing; none with no model.</summary>
        private static void MakeLook(Missile missile, IPhysicsWorld world, float x, float y)
        {
            if (missile.model.Length == 0)
            {
                return;
            }

            var look = world.AddEffect(missile.model, x, y);
            missile.look = look;
            world.SetEffectPosition(look, x, y, missile.z + missile.modelHeight);
            if (missile.scale != 1.0f)
            {
                world.SetEffectScale(look, missile.scale);
            }

            if (missile.tinted)
            {
                Colour(missile, look);
            }

            Turn(missile, world, look);
        }

        /// <summary>Makes the effect AddEffect listed at an index, at a place, at its height, with the missile's facing.</summary>
        private static void ShowExtra(Missile missile, int index, float x, float y, IPhysicsWorld world)
        {
            var made = world.AddEffect(missile.extraModels![index], x, y);
            world.SetEffectPosition(made, x, y, missile.z + missile.extraHeights![index]);
            missile.extras!.Add(made);
            Turn(missile, world, made);
        }

        /// <summary>Turns a new effect to the missile's facing: the last it was turned to, or its velocity when it has not been turned and has one.</summary>
        private static void Turn(Missile missile, IPhysicsWorld world, effect target)
        {
            if (!missile.facesFlight)
            {
                return;
            }

            if (!missile.faced)
            {
                Face(missile, world);
                return;
            }

            world.SetEffectOrientation(target, missile.facedYaw, missile.facedPitch, 0.0f);
        }
    }
}
