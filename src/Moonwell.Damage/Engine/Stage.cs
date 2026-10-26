namespace Moonwell
{
    /// <summary>Where a hit is on its way, in order; the first five are also the slots of the reaction methods.</summary>
    internal enum Stage
    {
        /// <summary>The game's damaging event: before armor.</summary>
        BeforeArmor = 0,

        /// <summary>The game's damaged event: after armor.</summary>
        AfterArmor = 1,

        /// <summary>After armor, when the hit would kill.</summary>
        Lethal = 2,

        /// <summary>0 s later, when the life has dropped.</summary>
        Landed = 3,

        /// <summary>0 s later, once per source: no hit is in it; it is the slot of the burst reactions.</summary>
        Burst = 4,

        /// <summary>Between the two events: the before-armor stage has ended, so nothing about the hit may change.</summary>
        Waiting = 5,

        /// <summary>Ended before armor: spell immunity stopped it, or its after-armor event never came.</summary>
        Stopped = 6,
    }
}
