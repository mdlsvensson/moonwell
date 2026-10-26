namespace Moonwell
{
    /// <summary>
    /// A unit's body as the physics package sees it: how heavy it is, how tall, how fast a push on it slows, and whether it
    /// can be pushed at all. A class lists it in its constructor (<c>Components.Add(new Body { Mass = 40f })</c>); a unit
    /// without one, or a value left unset, has the defaults: its collision size, 4 times that, 600 a second, movable. A
    /// structure is never pushed, whatever its Body says.
    /// </summary>
    public sealed class Body : UnitComponent
    {
        /// <summary>How many times its collision size a unit is tall unless Height is set.</summary>
        internal const float HeightPerSize = 4.0f;

        /// <summary>How fast a push slows unless Friction is set, in game units a second each second (ALICE's).</summary>
        internal const float DefaultFriction = 600.0f;

        /// <summary>The mass set; read only when massSet.</summary>
        private float mass;

        /// <summary>Whether Mass has been set.</summary>
        private bool massSet;

        /// <summary>The height set; read only when heightSet.</summary>
        private float height;

        /// <summary>Whether Height has been set.</summary>
        private bool heightSet;

        /// <summary>The friction set; read only when frictionSet.</summary>
        private float friction;

        /// <summary>Whether Friction has been set.</summary>
        private bool frictionSet;

        /// <summary>Whether Immovable was set to true.</summary>
        private bool immovable;

        /// <summary>How heavy the unit is, which shares out a collision's speed between it and what hit it; its collision size unless set (a footman 16). 0 on a Body that is on no unit yet.</summary>
        [LocalSafe]
        public float Mass
        {
            get { return massSet ? mass : Size(); }
            set
            {
                mass = value;
                massSet = true;
            }
        }

        /// <summary>How tall the unit is, from the ground plus its fly height, for what a missile meets; 4 times its collision size unless set. 0 on a Body that is on no unit yet.</summary>
        [LocalSafe]
        public float Height
        {
            get { return heightSet ? height : HeightPerSize * Size(); }
            set
            {
                height = value;
                heightSet = true;
            }
        }

        /// <summary>How fast a push on the unit slows, in game units a second each second, unless a Knockback set its own; 600 unless set. 0 lets a push slide on until something stops it. A flying unit pushed up or down drifts back to its height at this rate.</summary>
        [LocalSafe]
        public float Friction
        {
            get { return frictionSet ? friction : DefaultFriction; }
            set
            {
                friction = value;
                frictionSet = true;
            }
        }

        /// <summary>Whether the unit cannot be pushed: Push and Knockback do nothing to it. False unless set; always true on a structure.</summary>
        [LocalSafe]
        public bool Immovable
        {
            get { return immovable || IsStructure(); }
            set { immovable = value; }
        }

        /// <summary>The collision size of the unit the Body is on; 0 when it is on none or its unit is gone.</summary>
        private float Size()
        {
            var unit = actor as Unit;
            if (unit == null || unit.Handle == null || unit.IsGone)
            {
                return 0.0f;
            }

            return Physics.World.CollisionSize(unit.Handle);
        }

        /// <summary>Whether the Body is on a structure.</summary>
        private bool IsStructure()
        {
            var unit = actor as Unit;
            return unit != null && unit.Handle != null && !unit.IsGone && Physics.World.IsStructure(unit.Handle);
        }
    }
}
