namespace Moonwell
{
    /// <summary>What a missile does on touching a kind of thing: a setting of the missile for each kind, as its GroundContact.</summary>
    public enum Contact
    {
        /// <summary>It is not checked against that kind at all; for the ground, it flies through.</summary>
        None,

        /// <summary>It ends there.</summary>
        Stop,

        /// <summary>It bounces off, keeping its Bounciness of the speed into the surface.</summary>
        Bounce,

        /// <summary>It reacts on entering and flies on. The ground has no such outcome: there it flies through, as None.</summary>
        Pass,

        /// <summary>Ground and walls only: it bounces until slow, then rolls along the ground, or slides along the wall.</summary>
        Slide,
    }
}
