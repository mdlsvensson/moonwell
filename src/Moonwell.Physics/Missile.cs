using System.Collections.Generic;
using Moonwell.Engine;

namespace Moonwell
{
    /// <summary>
    /// A map's own object that flies: a bolt, a shell, a boulder. Every missile moves the same way, once a step: its speed
    /// changes by its Acceleration, gravity pulls it down, and it moves by its velocity. Its settings are declared in the
    /// constructor and are live after. A unit fires one with <c>unit.Fire(missile, target)</c>; <c>Missile.Create</c> places
    /// one for code that sets its velocity itself. It belongs to the game unless it is given an owner, and ends when it
    /// stops on the ground or on a thing it meets (its GroundContact, UnitContact and the rest), when its Duration or
    /// MaxDistance runs out, when it leaves the world, by End, or with its owner.
    /// </summary>
    public abstract class Missile : Gizmo
    {
        // The fields are internal, not private, as in Gizmo: a map's subclass may name a field of its own the same.

        /// <summary>The velocity along x, in game units a second.</summary>
        internal float vx;

        /// <summary>The velocity along y, in game units a second.</summary>
        internal float vy;

        /// <summary>The velocity along z, in game units a second.</summary>
        internal float vz;

        /// <summary>The speed added along the flight each second.</summary>
        internal float acceleration;

        /// <summary>The speed Acceleration stops at; 0 for none.</summary>
        internal float maxSpeed;

        /// <summary>The pull down, in game units a second each second.</summary>
        internal float gravity;

        /// <summary>The peak of a fired arc as a share of the distance; 0 for none.</summary>
        internal float arc;

        /// <summary>Whether the missile keeps its height above the ground.</summary>
        internal bool followsGround;

        /// <summary>The radius of the missile's ball.</summary>
        internal float radius = 16.0f;

        /// <summary>How hard it shoves what it hits.</summary>
        internal float mass;

        /// <summary>The share of the speed into a surface a bounce keeps.</summary>
        internal float bounciness = 0.5f;

        /// <summary>The speed lost a second while sliding.</summary>
        internal float friction = 200.0f;

        /// <summary>The seconds it flies before it expires; 0 for no end.</summary>
        internal float duration;

        /// <summary>The distance it flies before it expires; 0 for no end.</summary>
        internal float maxDistance;

        /// <summary>The unit that fired it, or null.</summary>
        internal Unit? source;

        /// <summary>The damage at a contact.</summary>
        internal float impactDamage;

        /// <summary>The kind of the damage at a contact.</summary>
        internal DamageKind damageKind = DamageKind.Magic;

        /// <summary>The attack type of the damage at a contact.</summary>
        internal AttackType attackType = AttackType.Spells;

        /// <summary>Whether the damage at a contact is an attack.</summary>
        internal bool isAttack;

        /// <summary>Whether the damage at a contact is ranged.</summary>
        internal bool ranged;

        /// <summary>The model of the missile's effect; empty for none.</summary>
        internal string model = "";

        /// <summary>The scale of the missile's effect.</summary>
        internal float scale = 1.0f;

        /// <summary>How far above its place the effect is drawn.</summary>
        internal float modelHeight;

        /// <summary>Whether the effects turn to the velocity.</summary>
        internal bool facesFlight = true;

        /// <summary>How far above the source's ground and fly height Fire starts it.</summary>
        internal float launchHeight = 60.0f;

        /// <summary>How far ahead of the source Fire starts it.</summary>
        internal float launchOffset;

        /// <summary>The model played once where it stops; empty for none.</summary>
        internal string impactModel = "";

        /// <summary>The sound played once where it stops; empty for none.</summary>
        internal string impactSound = "";

        /// <summary>The effect of the model; null with no model, before Create, and after the end.</summary>
        internal effect? look;

        /// <summary>Whether the effects have been made: from Create until the end.</summary>
        internal bool shown;

        /// <summary>Whether Missile.Create gave the height; a missile placed through Gizmo.Create starts at its LaunchHeight above the ground.</summary>
        internal bool heightGiven;

        /// <summary>The models of the effects added by AddEffect, in the order added.</summary>
        internal List<string>? extraModels;

        /// <summary>The heights above its place of the effects added by AddEffect, in the order added.</summary>
        internal List<float>? extraHeights;

        /// <summary>The effects added by AddEffect, in the order added; made at Create, or at once after it.</summary>
        internal List<effect>? extras;

        /// <summary>Whether Tint has been called.</summary>
        internal bool tinted;

        /// <summary>The red of the tint.</summary>
        internal int tintRed = 255;

        /// <summary>The green of the tint.</summary>
        internal int tintGreen = 255;

        /// <summary>The blue of the tint.</summary>
        internal int tintBlue = 255;

        /// <summary>The opacity of the tint.</summary>
        internal int tintAlpha = 255;

        /// <summary>Whether the effects have been turned to a velocity.</summary>
        internal bool faced;

        /// <summary>The velocity along x the effects were last turned to.</summary>
        internal float facedX;

        /// <summary>The velocity along y the effects were last turned to.</summary>
        internal float facedY;

        /// <summary>The velocity along z the effects were last turned to.</summary>
        internal float facedZ;

        /// <summary>The yaw the effects were last turned to, in radians.</summary>
        internal float facedYaw;

        /// <summary>The pitch the effects were last turned to, in radians.</summary>
        internal float facedPitch;

        /// <summary>The seconds it has flown.</summary>
        internal float flown;

        /// <summary>The distance it has flown.</summary>
        internal float travelled;

        /// <summary>Whether the ground below was read for the place in groundX and groundY.</summary>
        internal bool groundRead;

        /// <summary>The x the ground was last read at.</summary>
        internal float groundX;

        /// <summary>The y the ground was last read at.</summary>
        internal float groundY;

        /// <summary>The ground's height last read; while groundPending, not read yet (Ground.Last reads it).</summary>
        internal float groundZ;

        /// <summary>Whether the last read of the ground, at groundX and groundY, was put off as it could only find the ground below the missile: spanGrid reads it when something needs its height.</summary>
        internal bool groundPending;

        /// <summary>The seconds since the ground was last read, for the ground's contact.</summary>
        internal float groundSince;

        /// <summary>The terrain's grid the span was read from; null before the first.</summary>
        internal HeightGrid? spanGrid;

        /// <summary>The span's box (HeightGrid.Span), where the ground is between spanLow and spanHigh: its left edge.</summary>
        internal float spanMinX;

        /// <summary>The span's bottom edge.</summary>
        internal float spanMinY;

        /// <summary>The span's right edge; below spanMinX for no box.</summary>
        internal float spanMaxX = -1.0f;

        /// <summary>The span's top edge.</summary>
        internal float spanMaxY = -1.0f;

        /// <summary>The ground in the span's box is above this.</summary>
        internal float spanLow;

        /// <summary>The ground in the span's box is below this.</summary>
        internal float spanHigh;

        /// <summary>What it does on meeting the ground.</summary>
        internal Contact groundContact = Contact.Stop;

        /// <summary>What it does on meeting a unit.</summary>
        internal Contact unitContact = Contact.Stop;

        /// <summary>What it does on meeting a destructable.</summary>
        internal Contact destructableContact;

        /// <summary>What it does on meeting an item.</summary>
        internal Contact itemContact;

        /// <summary>What it does on meeting another missile.</summary>
        internal Contact missileContact;

        /// <summary>What it does on meeting a wall.</summary>
        internal Contact wallContact;

        /// <summary>The interaction that checks it against units; null while UnitContact has been None since Create.</summary>
        internal Interaction? unitChecks;

        /// <summary>The interaction that checks it against destructables; null while DestructableContact has been None since Create.</summary>
        internal Interaction? destructableChecks;

        /// <summary>The interaction that checks it against items; null while ItemContact has been None since Create.</summary>
        internal Interaction? itemChecks;

        /// <summary>The interaction that checks it against other missiles; null while MissileContact has been None since Create.</summary>
        internal Interaction? missileChecks;

        /// <summary>What the package keeps of each wall its ball is touching or near, in the order first met; null until the first. A wall left behind is taken out.</summary>
        internal List<Touch>? wallTouches;

        /// <summary>What it keeps with each other missile it is paired with, either side's, in the order they met; null until the first. Taken out once both sides have parted.</summary>
        internal List<Touch>? meetTouches;

        /// <summary>Its number in the order missiles were made since the install, from 1: of two missiles, the one made first meets the other.</summary>
        internal int made;

        /// <summary>Its radius plus how far it moved along the ground this step, for the range of the checks against other missiles.</summary>
        internal float meetReach;

        /// <summary>Where the step's move began, x: the contact checks sweep its ball from there to where it is.</summary>
        internal float lastX;

        /// <summary>Where the step's move began, y.</summary>
        internal float lastY;

        /// <summary>Where the step's move began, its height.</summary>
        internal float lastZ;

        /// <summary>The step of the cycle its move began at (the pass's step, so the move covers that step); one woken from rest keeps the step before, covering two, so two missiles' moves can be put in one time frame.</summary>
        internal int lastStep;

        /// <summary>Where its flight or its contacts last put it, x: a check that finds it elsewhere knows its X, Y or Z was set from outside, and starts its sweep there instead of sweeping along the jump.</summary>
        internal float placedX;

        /// <summary>Where its flight or its contacts last put it, y.</summary>
        internal float placedY;

        /// <summary>Where its flight or its contacts last put it, its height.</summary>
        internal float placedZ;

        /// <summary>Whether the next step keeps the place its move began from: a missile woken from rest, whose checks resume a step late, so the next check sweeps both steps.</summary>
        internal bool keepLast;

        /// <summary>Whether it rolls along the ground: a Slide that has slowed.</summary>
        internal bool rolling;

        /// <summary>Whether its move this step was a free fall, gravity taken for the whole step: a contact later in the step gives back the part after it. False once it met the ground or rolled along it this step, whose velocity is the ground's.</summary>
        internal bool fell;

        /// <summary>Whether it lies still on the ground; it moves again when something gives it a velocity.</summary>
        internal bool resting;

        /// <summary>Where it came to rest: x. Moved from its rest place, it wakes.</summary>
        internal float restX;

        /// <summary>Where it came to rest: y.</summary>
        internal float restY;

        /// <summary>Where it came to rest: its height.</summary>
        internal float restZ;

        /// <summary>Whether the deck it rests on has gone since it came to rest (its bridge died): it wakes, to fall.</summary>
        internal bool groundGone;

        /// <summary>Whether a reaction let it through the ground: it is not met again until it is above the ground.</summary>
        internal bool throughGround;

        /// <summary>The Homing that turns it: the first added that is still on it; null for none.</summary>
        internal Homing? homing;

        /// <summary>The shadows on it, in the order added; null until the first. Moved with its effects.</summary>
        internal List<Shadow>? shadows;

        /// <summary>The looping sounds on it, in the order added; null until the first. Moved with its effects.</summary>
        internal List<LoopingSound>? sounds;

        /// <summary>The velocity along x, in game units a second. Set faster, the checks waiting on it look again at their next tick.</summary>
        [LocalSafe]
        public float VX
        {
            get { return vx; }
            set
            {
                var before = vx * vx + vy * vy + vz * vz;
                vx = value;
                Contacts.Quicker(this, before);
            }
        }

        /// <summary>The velocity along y, in game units a second.</summary>
        [LocalSafe]
        public float VY
        {
            get { return vy; }
            set
            {
                var before = vx * vx + vy * vy + vz * vz;
                vy = value;
                Contacts.Quicker(this, before);
            }
        }

        /// <summary>The velocity along z, in game units a second; up is positive.</summary>
        [LocalSafe]
        public float VZ
        {
            get { return vz; }
            set
            {
                var before = vx * vx + vy * vy + vz * vz;
                vz = value;
                Contacts.Quicker(this, before);
            }
        }

        /// <summary>The velocity as a vector; a vector is a table, so code that runs every step on many missiles reads VX, VY and VZ. Setting it sets all three.</summary>
        [LocalSafe]
        public Vector3 Velocity
        {
            get { return new Vector3(vx, vy, vz); }
            set
            {
                var before = vx * vx + vy * vy + vz * vz;
                vx = value.X;
                vy = value.Y;
                vz = value.Z;
                Contacts.Quicker(this, before);
            }
        }

        /// <summary>The length of the velocity, in game units a second; 0 by default. Setting it keeps the direction, or goes along x when the missile is still. Fire launches the missile at it.</summary>
        [LocalSafe]
        public float Speed
        {
            get { return Trig.Sqrt(vx * vx + vy * vy + vz * vz); }
            set
            {
                Dev.Check(value >= 0.0f, Names.NameOf(this) + ".Speed: must be 0 or more");
                var before = vx * vx + vy * vy + vz * vz;
                var now = Trig.Sqrt(before);
                if (now > 0.0f)
                {
                    var share = value / now;
                    vx *= share;
                    vy *= share;
                    vz *= share;
                }
                else
                {
                    vx = value;
                    vy = 0.0f;
                    vz = 0.0f;
                }

                Contacts.Quicker(this, before);
            }
        }

        /// <summary>The speed added along the flight each second, up to MaxSpeed; a negative one slows it to a stop. 0 by default. A still missile has no flight to add along.</summary>
        [LocalSafe]
        public float Acceleration
        {
            get { return acceleration; }
            set { acceleration = value; }
        }

        /// <summary>The speed Acceleration adds up to; 0, the default, for no cap.</summary>
        [LocalSafe]
        public float MaxSpeed
        {
            get { return maxSpeed; }
            set { maxSpeed = value; }
        }

        /// <summary>How much it is pulled down a second each second; 0 by default. Physics.Gravity (800) is the world's. Fire with an Arc sets it.</summary>
        [LocalSafe]
        public float Gravity
        {
            get { return gravity; }
            set { gravity = value; }
        }

        /// <summary>For Fire at a point or a unit: the peak's height as a share of the distance, the game's Missile Arc; 0, the default, for none. Fire then flies it along the ground at Speed and works out its Gravity and lift so that it lands on the mark.</summary>
        [LocalSafe]
        public float Arc
        {
            get { return arc; }
            set { arc = value; }
        }

        /// <summary>Whether it keeps its height above the ground instead of its height, as a shockwave; false by default.</summary>
        [LocalSafe]
        public bool FollowsGround
        {
            get { return followsGround; }
            set { followsGround = value; }
        }

        /// <summary>Its size: a ball of this radius; 16 by default.</summary>
        [LocalSafe]
        public float Radius
        {
            get { return radius; }
            set { radius = value; }
        }

        /// <summary>How hard it shoves what it hits; 0, the default, shoves nothing.</summary>
        [LocalSafe]
        public float Mass
        {
            get { return mass; }
            set { mass = value; }
        }

        /// <summary>0 to 1: the share of its speed into a surface a bounce keeps; 0.5 by default.</summary>
        [LocalSafe]
        public float Bounciness
        {
            get { return bounciness; }
            set { bounciness = value; }
        }

        /// <summary>The speed it loses a second while sliding on the ground: a Slide rolling, or a Bounce going along the ground once it no longer bounces. Stopped where a slope's pull is no more than this, it rests and OnRest runs. 200 by default; 0 never slows it.</summary>
        [LocalSafe]
        public float Friction
        {
            get { return friction; }
            set { friction = value; }
        }

        /// <summary>What it does on meeting the ground (water's surface and bridges' decks too): Stop, the default, ends it there; Bounce bounces off, and once too slow to bounce goes along the ground, slowing by its Friction, and comes to rest as Slide does; Slide bounces until slow, then rolls and comes to rest; None flies through, and so does Pass, as the ground has no such outcome.</summary>
        [LocalSafe]
        public Contact GroundContact
        {
            get { return groundContact; }
            set { groundContact = value; }
        }

        /// <summary>What it does on meeting a unit: Stop, the default, ends it there; Bounce bounces it off (Slide too); Pass reacts once and flies on, not again until it has left the unit; None does not check units at all. Set to another than None after Create, the checks start (or resume) then; set to None, they pause.</summary>
        [LocalSafe]
        public Contact UnitContact
        {
            get { return unitContact; }
            set
            {
                unitContact = value;
                Contacts.Changed(this);
            }
        }

        /// <summary>What it does on meeting a destructable: None, the default, does not check destructables at all; Stop, Bounce and Pass as UnitContact. Set to another than None after Create, the checks start then.</summary>
        [LocalSafe]
        public Contact DestructableContact
        {
            get { return destructableContact; }
            set
            {
                destructableContact = value;
                Contacts.Changed(this);
            }
        }

        /// <summary>What it does on meeting an item on the ground: None, the default, does not check items at all; Stop, Bounce and Pass as UnitContact. An item takes no damage. Set to another than None after Create, the checks start then.</summary>
        [LocalSafe]
        public Contact ItemContact
        {
            get { return itemContact; }
            set
            {
                itemContact = value;
                Contacts.Changed(this);
            }
        }

        /// <summary>What it does on meeting another missile: None, the default, does not check missiles at all and reacts to none (a missile that checks missiles still meets it, and it flies on as it was); Stop, Bounce and Pass as UnitContact (Slide is Bounce). Two missiles meet once, through the one made first; each takes its own outcome, and a bounce keeps the two Bouncinesses multiplied and square-rooted, shared by Mass. Set to another than None after Create, the checks start then.</summary>
        [LocalSafe]
        public Contact MissileContact
        {
            get { return missileContact; }
            set
            {
                missileContact = value;
                Contacts.Changed(this);
            }
        }

        /// <summary>What it does on meeting a wall: None, the default, does not check walls at all; Stop and Pass as UnitContact; Bounce bounces off, keeping its Bounciness times the wall's; Slide bounces so until it meets the wall slower than 5, then loses the speed into it and slides along it, keeping the rest.</summary>
        [LocalSafe]
        public Contact WallContact
        {
            get { return wallContact; }
            set
            {
                wallContact = value;
                Contacts.Changed(this);
            }
        }

        /// <summary>The seconds it flies, counted from Create; then OnExpire runs and it ends. 0, the default, for no end.</summary>
        [LocalSafe]
        public float Duration
        {
            get { return duration; }
            set { duration = value; }
        }

        /// <summary>The distance it flies, counted from Create along its path; then OnExpire runs and it ends. 0, the default, for no end.</summary>
        [LocalSafe]
        public float MaxDistance
        {
            get { return maxDistance; }
            set { maxDistance = value; }
        }

        /// <summary>The unit that fired it, or null; Fire sets it. The unit may die while the missile flies.</summary>
        [LocalSafe]
        public Unit? Source
        {
            get { return source; }
            set { source = value; }
        }

        /// <summary>The damage its Source deals at a contact; 0, the default, for none.</summary>
        [LocalSafe]
        public float ImpactDamage
        {
            get { return impactDamage; }
            set { impactDamage = value; }
        }

        /// <summary>The kind of the damage at a contact; Magic by default, as a unit's Damage.</summary>
        [LocalSafe]
        public DamageKind DamageKind
        {
            get { return damageKind; }
            set { damageKind = value; }
        }

        /// <summary>The attack type of the damage at a contact; Spells by default, as a unit's Damage.</summary>
        [LocalSafe]
        public AttackType AttackType
        {
            get { return attackType; }
            set { attackType = value; }
        }

        /// <summary>Whether the damage at a contact is an attack; false by default, as a unit's Damage.</summary>
        [LocalSafe]
        public bool IsAttack
        {
            get { return isAttack; }
            set { isAttack = value; }
        }

        /// <summary>Whether the damage at a contact, as an attack, is ranged; false by default, as a unit's Damage.</summary>
        [LocalSafe]
        public bool Ranged
        {
            get { return ranged; }
            set { ranged = value; }
        }

        /// <summary>The model of its effect; empty, the default, for none. Set after Create, it replaces the effect.</summary>
        [LocalSafe]
        public string Model
        {
            get { return model; }
            set
            {
                model = value ?? "";
                if (shown)
                {
                    Flight.Remake(this);
                }
            }
        }

        /// <summary>The scale of its effect; 1 by default.</summary>
        [LocalSafe]
        public float Scale
        {
            get { return scale; }
            set
            {
                scale = value;
                if (look != null)
                {
                    Physics.World.SetEffectScale(look, value);
                }
            }
        }

        /// <summary>How far above its place its effect is drawn; 0 by default.</summary>
        [LocalSafe]
        public float ModelHeight
        {
            get { return modelHeight; }
            set { modelHeight = value; }
        }

        /// <summary>Whether its effects turn, yaw and pitch, to the velocity each step; true by default. A missile that is still keeps its last facing.</summary>
        [LocalSafe]
        public bool FacesFlight
        {
            get { return facesFlight; }
            set { facesFlight = value; }
        }

        /// <summary>How far above the ground and the source's fly height Fire starts it; 60 by default.</summary>
        [LocalSafe]
        public float LaunchHeight
        {
            get { return launchHeight; }
            set { launchHeight = value; }
        }

        /// <summary>How far ahead of the source, along its facing, Fire starts it; 0 by default.</summary>
        [LocalSafe]
        public float LaunchOffset
        {
            get { return launchOffset; }
            set { launchOffset = value; }
        }

        /// <summary>The model played once where it stops; empty, the default, for none.</summary>
        [LocalSafe]
        public string ImpactModel
        {
            get { return impactModel; }
            set { impactModel = value ?? ""; }
        }

        /// <summary>The sound played once where it stops; empty, the default, for none.</summary>
        [LocalSafe]
        public string ImpactSound
        {
            get { return impactSound; }
            set { impactSound = value ?? ""; }
        }

        /// <summary>The game's effect of its Model, for anything Moonwell does not do; null with no model, before Create and after the end.</summary>
        [LocalSafe]
        public effect? Effect
        {
            get { return look; }
        }

        /// <summary>Places a missile of the map at a place and height, still unless its velocity is set, for a player, and returns it. It belongs to the owner given, else to the game, not to the current owner, so a missile made in a spell's reaction outlives the cast. The player is Neutral Passive unless given. OnCreated runs with the missile as the current owner. Placed with Gizmo.Create, or with no height, a missile is the same, at its LaunchHeight above the ground.</summary>
        public static T Create<T>(T missile, float x, float y, float z, Player? player = null, Owner? owner = null)
            where T : Missile
        {
            Dev.Check(missile.entry == null, Names.NameOf(missile) + ".Create: the missile has been created already");
            Physics.EnsureInstalled();
            missile.z = z;
            missile.heightGiven = true;
            Gizmo.Create(missile, x, y, player, owner);
            return missile;
        }

        /// <summary>Tints its effect: red, green and blue from 0 to 255, and the opacity, 255 unless given.</summary>
        public void Tint(int r, int g, int b, int a = 255)
        {
            tinted = true;
            tintRed = r;
            tintGreen = g;
            tintBlue = b;
            tintAlpha = a;
            if (look != null)
            {
                Flight.Colour(this, look);
            }
        }

        /// <summary>Adds an effect of a model that follows the missile, drawn a height above its place, and turns with it; it is destroyed when the missile ends. Before Create it is made at Create.</summary>
        public void AddEffect(string model, float height)
        {
            if (extraModels == null)
            {
                extraModels = new List<string>();
                extraHeights = new List<float>();
                extras = new List<effect>();
            }

            extraModels.Add(model);
            extraHeights!.Add(height);
            if (shown)
            {
                Flight.ShowExtra(this, extraModels.Count - 1);
            }
        }

        /// <summary>Whether it can hit a thing it touches: asked before a contact with a unit, destructable, item, missile (each of the two is asked about the other) or wall, with the missile as the current owner; false and they pass through each other, and it is not asked again until they part. By default everything but its own Source.</summary>
        public virtual bool CanHit(Actor other)
        {
            return other != source;
        }

        /// <summary>Runs at each contact, before the damage and the outcome, with the missile's owner as the current owner, so what it makes outlives a missile that stops there.</summary>
        public virtual void OnImpact(Impact impact)
        {
        }

        /// <summary>Runs once it lies still on the ground, with the missile as the current owner.</summary>
        public virtual void OnRest()
        {
        }

        /// <summary>Runs when its Duration or MaxDistance has run out, just before it ends, with the missile as the current owner.</summary>
        public virtual void OnExpire()
        {
        }

        /// <summary>The components the class starts with, for Fire to give a Homing its target before Create adds them.</summary>
        internal List<GizmoComponent> StartingComponents()
        {
            return Components;
        }

        /// <summary>A missile given no owner belongs to the game, not to the current owner, so one made in a spell's reaction outlives the cast (decision 59).</summary>
        internal override Owner DefaultOwner()
        {
            return Game.Instance;
        }

        /// <summary>Joins the physics pass, makes its effects and starts its contact checks, as Gizmo.Create places it: at the height Missile.Create gave, else at its LaunchHeight above the ground.</summary>
        internal override void Placed()
        {
            Physics.EnsureInstalled();
            var x = entry!.X;
            var y = entry.Y;
            if (!heightGiven)
            {
                z = Ground.Height(x, y) + launchHeight;
            }

            lastX = x;
            lastY = y;
            lastZ = z;
            lastStep = Tracking.StepNow;
            placedX = x;
            placedY = y;
            placedZ = z;
            Flight.Show(this, x, y);
            Pass.Add(this);
            Contacts.Begin(this);
        }

        /// <summary>Destroys its effects once it and its components have ended, and tells the pass it is gone.</summary>
        internal override void Finish()
        {
            base.Finish();
            Flight.Hide(this);
            Pass.Dropped();
        }
    }
}
