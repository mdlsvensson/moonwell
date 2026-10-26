using Moonwell.Engine;

namespace Moonwell
{
    /// <summary>
    /// Turns a missile toward a target as it flies: <c>Components.Add(new Homing { TurnRate = 180f })</c>, and
    /// <c>unit.Fire(missile, target)</c> gives it its target. Every 0.1 second it turns the missile's velocity toward the
    /// target's middle by at most TurnRate a second, keeping its speed. While the target is inside the circle the missile
    /// cannot turn tight enough to reach, it flies straight on to gain room instead of circling it (ALICE's way). With an
    /// Arc it re-aims the arc's rest at the target each time instead, keeping its speed along the ground and its Gravity,
    /// so that it comes down on a target that walks. When the target dies or leaves the game, or a gizmo target ends, it is
    /// let go at the next turn: the missile flies on straight and Target reads null, or it ends with EndsWithTarget. A
    /// missile is turned by the first Homing on it.
    /// </summary>
    public sealed class Homing : MissileComponent
    {
        /// <summary>The seconds between two turns (ALICE's).</summary>
        internal const float Interval = 0.1f;

        /// <summary>Once turning, a missile goes on turning while the target is more than this share of its turning circle's radius from the circle's centre (ALICE's hold), so it does not stop and start at the circle's edge.</summary>
        private const float Hold = 0.9f;

        /// <summary>A target nearer than this, along the ground for an arc, is at the missile's own place: it is not turned.</summary>
        private const float Near = 1.0f;

        /// <summary>The sine under which the target is straight ahead of or behind the flight.</summary>
        private const float Ahead = 0.0001f;

        /// <summary>The share of a step a turn's interval is forgiven, as a sum of steps can come out a little under in a float.</summary>
        private const float Forgiven = 0.001f;

        /// <summary>Radians in a degree.</summary>
        private const float RadiansPerDegree = 0.017453292f;

        /// <summary>The target; null for none, and once it is lost.</summary>
        private Actor? target;

        /// <summary>The most it turns a second, in degrees; 0 for at once.</summary>
        private float turnRate;

        /// <summary>Whether the missile ends when the target is lost.</summary>
        private bool endsWithTarget;

        /// <summary>The seconds counted toward the next turn; it starts full, so the first step turns.</summary>
        internal float since = Interval;

        /// <summary>Whether it turned at its last turn with the target near: it then turns on while the target is outside the hold.</summary>
        internal bool turning;

        /// <summary>The unit or gizmo it turns toward; null for none. Fire at a unit sets it. It reads null once the target has died, left the game or ended.</summary>
        [LocalSafe]
        public Actor? Target
        {
            get { return target; }
            set
            {
                Dev.Check(value == null || value is Unit || value is Gizmo, "Homing.Target: a unit or a gizmo");
                target = value;
                turning = false;
            }
        }

        /// <summary>The most it turns a second, in degrees; 0, the default, turns it at the target at once.</summary>
        [LocalSafe]
        public float TurnRate
        {
            get { return turnRate; }
            set
            {
                Dev.Check(value >= 0.0f, "Homing.TurnRate: degrees a second must be 0 or more");
                turnRate = value;
            }
        }

        /// <summary>Whether the missile ends when its target dies, leaves the game or ends; false, the default, flies it on straight.</summary>
        [LocalSafe]
        public bool EndsWithTarget
        {
            get { return endsWithTarget; }
            set { endsWithTarget = value; }
        }

        /// <summary>Becomes the missile's Homing unless it has one.</summary>
        protected override void OnAdded()
        {
            var missile = Missile;
            if (missile.homing == null)
            {
                missile.homing = this;
            }
        }

        /// <summary>Hands the turning to the next Homing on the missile, if there is one.</summary>
        protected override void OnRemoved()
        {
            var missile = Missile;
            if (missile.homing != this)
            {
                return;
            }

            missile.homing = null;
            var components = missile.components;
            for (var i = 0; i < components.Count; i++)
            {
                var other = components[i] as Homing;
                if (other != null && other != this && !other.ending && !other.HasEnded)
                {
                    missile.homing = other;
                    return;
                }
            }
        }

        /// <summary>
        /// The pass's turn, before a missile's move, with its velocity as it would fly this step in its vx, vy and vz and its
        /// place at the step's start. Counts the step, and every Interval turns the velocity; true when it turned it or ended
        /// the missile (EndsWithTarget). Makes no table.
        /// </summary>
        internal bool Turn(Missile missile, float step, float x, float y, float z)
        {
            var counted = since + step;
            if (counted < Interval - step * Forgiven)
            {
                since = counted;
                return false;
            }

            counted -= Interval;
            since = counted > Interval ? 0.0f : counted;
            var aim = target;
            if (aim == null)
            {
                return false;
            }

            float tx;
            float ty;
            float tz;
            if (!Locate(aim, out tx, out ty, out tz))
            {
                target = null;
                turning = false;
                if (endsWithTarget)
                {
                    missile.End();
                    return true;
                }

                return false;
            }

            if (missile.arc > 0.0f)
            {
                return Reaim(missile, step, tx - x, ty - y, tz - z);
            }

            return Steer(missile, tx - x, ty - y, tz - z);
        }

        /// <summary>
        /// A target's middle: a unit's place, half its height above the ground and its fly height; a gizmo's place and
        /// height. False, and nothing of it read, once a unit has left the game or died or a gizmo has ended.
        /// </summary>
        private static bool Locate(Actor aim, out float x, out float y, out float z)
        {
            var unit = aim as Unit;
            if (unit != null)
            {
                // IsGone first: a unit the game has removed is not asked anything.
                if (unit.IsGone || unit.ending || unit.HasEnded || !unit.IsAlive)
                {
                    x = 0.0f;
                    y = 0.0f;
                    z = 0.0f;
                    return false;
                }

                x = unit.X;
                y = unit.Y;
                z = Ground.Height(x, y) + unit.FlyHeight + Shapes.UnitHeight(unit) * 0.5f;
                return true;
            }

            var gizmo = aim as Gizmo;
            if (gizmo != null && gizmo.entry != null && !gizmo.ending && !gizmo.HasEnded)
            {
                x = gizmo.entry.X;
                y = gizmo.entry.Y;
                z = gizmo.z;
                return true;
            }

            x = 0.0f;
            y = 0.0f;
            z = 0.0f;
            return false;
        }

        /// <summary>
        /// Turns the velocity toward a target a way off by at most TurnRate times Interval, in the plane of the velocity and
        /// the target, keeping the speed; at once when it is within that, or TurnRate is 0. A target inside the circle it
        /// cannot turn into leaves it straight. A still missile, or a target at its place, is not turned.
        /// </summary>
        private bool Steer(Missile missile, float dx, float dy, float dz)
        {
            var vx = missile.vx;
            var vy = missile.vy;
            var vz = missile.vz;
            var speed = Trig.Sqrt(vx * vx + vy * vy + vz * vz);
            var distance = Trig.Sqrt(dx * dx + dy * dy + dz * dz);
            if (speed <= 0.0f || distance < Near)
            {
                return false;
            }

            var ux = vx / speed;
            var uy = vy / speed;
            var uz = vz / speed;
            var ex = dx / distance;
            var ey = dy / distance;
            var ez = dz / distance;
            var cos = ux * ex + uy * ey + uz * ez;

            // The direction across the flight toward the target, of length 1.
            var px = ex - cos * ux;
            var py = ey - cos * uy;
            var pz = ez - cos * uz;
            var sin = Trig.Sqrt(px * px + py * py + pz * pz);
            if (sin < Ahead)
            {
                if (cos > 0.0f)
                {
                    return false;
                }

                // Straight behind: it turns to its left along the ground, or along x when it flies straight up or down.
                var level = Trig.Sqrt(ux * ux + uy * uy);
                if (level > Ahead)
                {
                    px = -uy / level;
                    py = ux / level;
                }
                else
                {
                    px = 1.0f;
                    py = 0.0f;
                }

                pz = 0.0f;
                sin = 0.0f;
            }
            else
            {
                px /= sin;
                py /= sin;
                pz /= sin;
            }

            var angle = Trig.Atan2(sin, cos);
            var rate = turnRate * RadiansPerDegree;
            var most = rate * Interval;
            if (rate <= 0.0f || angle <= most)
            {
                missile.vx = ex * speed;
                missile.vy = ey * speed;
                missile.vz = ez * speed;
                return true;
            }

            var radius = speed / rate;
            if (distance < 2.0f * radius)
            {
                // The centre of the circle it would turn along; a target inside it cannot be reached by turning.
                var cx = radius * px - dx;
                var cy = radius * py - dy;
                var cz = radius * pz - dz;
                var gap = Trig.Sqrt(cx * cx + cy * cy + cz * cz);
                if (gap <= radius && !(turning && gap > Hold * radius))
                {
                    turning = false;
                    return false;
                }
            }

            turning = true;
            var c = Trig.Cos(most);
            var s = Trig.Sin(most);
            missile.vx = (c * ux + s * px) * speed;
            missile.vy = (c * uy + s * py) * speed;
            missile.vz = (c * uz + s * pz) * speed;
            return true;
        }

        /// <summary>
        /// Re-aims an arc at a target: its heading along the ground turns toward the target by at most TurnRate times
        /// Interval (at once with 0), at the same speed along the ground, and its speed up is set so that, with its Gravity,
        /// it comes down to the target's middle when it has crossed the distance there. A target nearer than a step's flight
        /// along the ground leaves it as it is.
        /// </summary>
        private bool Reaim(Missile missile, float step, float dx, float dy, float dz)
        {
            var vx = missile.vx;
            var vy = missile.vy;
            var along = Trig.Sqrt(vx * vx + vy * vy);
            var distance = Trig.Sqrt(dx * dx + dy * dy);
            if (along <= 0.0f || distance < Near || distance < along * step)
            {
                return false;
            }

            var hx = vx / along;
            var hy = vy / along;
            var ex = dx / distance;
            var ey = dy / distance;
            var rate = turnRate * RadiansPerDegree;
            if (rate > 0.0f)
            {
                var most = rate * Interval;
                var angle = Trig.Atan2(hx * ey - hy * ex, hx * ex + hy * ey);
                if (angle > most || angle < -most)
                {
                    var turn = angle > 0.0f ? most : -most;
                    var c = Trig.Cos(turn);
                    var s = Trig.Sin(turn);
                    ex = c * hx - s * hy;
                    ey = s * hx + c * hy;
                }
            }

            var gravity = missile.gravity;
            var time = distance / along;

            // The speed up at the step's start that lands it there; vz is the speed after this step's gravity.
            var lift = dz / time + gravity * time * 0.5f;
            missile.vx = ex * along;
            missile.vy = ey * along;
            missile.vz = lift - gravity * step;
            return true;
        }
    }
}
