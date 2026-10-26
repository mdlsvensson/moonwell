namespace Moonwell
{
    /// <summary>A missile's contact with something: what it met, where, how fast, and what follows. The package makes one at each contact and gives it to OnImpact, which may change its Contact.</summary>
    public sealed class Impact
    {
        /// <summary>What was met.</summary>
        internal ImpactKind kind;

        /// <summary>The point of contact: x.</summary>
        internal float x;

        /// <summary>The point of contact: y.</summary>
        internal float y;

        /// <summary>The point of contact: z.</summary>
        internal float z;

        /// <summary>The normal out of the surface: x.</summary>
        internal float nx;

        /// <summary>The normal out of the surface: y.</summary>
        internal float ny;

        /// <summary>The normal out of the surface: z.</summary>
        internal float nz;

        /// <summary>How fast they closed.</summary>
        internal float speed;

        /// <summary>The outcome.</summary>
        internal Contact contact;

        /// <summary>Made by the package at a contact only: what was met, the point, the normal out of the surface (of length 1), the closing speed and the outcome the missile's setting gives.</summary>
        internal Impact(ImpactKind kind, float x, float y, float z, float nx, float ny, float nz, float speed, Contact contact)
        {
            this.kind = kind;
            this.x = x;
            this.y = y;
            this.z = z;
            this.nx = nx;
            this.ny = ny;
            this.nz = nz;
            this.speed = speed;
            this.contact = contact;
        }

        /// <summary>What the missile met.</summary>
        [LocalSafe]
        public ImpactKind Kind
        {
            get { return kind; }
        }

        /// <summary>The point of contact's left-to-right place.</summary>
        [LocalSafe]
        public float X
        {
            get { return x; }
        }

        /// <summary>The point of contact's bottom-to-top place.</summary>
        [LocalSafe]
        public float Y
        {
            get { return y; }
        }

        /// <summary>The point of contact's height.</summary>
        [LocalSafe]
        public float Z
        {
            get { return z; }
        }

        /// <summary>The direction straight out of the surface met, of length 1; a vector is a table, made when read. On the ground in a cliff tile it is level, as off a wall.</summary>
        [LocalSafe]
        public Vector3 Normal
        {
            get { return new Vector3(nx, ny, nz); }
        }

        /// <summary>How fast the two closed, along the normal, in game units a second.</summary>
        [LocalSafe]
        public float Speed
        {
            get { return speed; }
        }

        /// <summary>The outcome: the missile's setting for the kind met. A reaction may change it, and the outcome changes with it.</summary>
        [LocalSafe]
        public Contact Contact
        {
            get { return contact; }
            set { contact = value; }
        }
    }
}
