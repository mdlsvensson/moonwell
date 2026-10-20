namespace Moonwell
{
    /// <summary>The stages a hit goes through, in order.</summary>
    internal enum Stage
    {
        /// <summary>The game's damaging event: before armor.</summary>
        BeforeArmor,

        /// <summary>The game's damaged event: after armor.</summary>
        AfterArmor,

        /// <summary>After armor, when the hit would kill.</summary>
        Lethal,

        /// <summary>0 s later, when the life has dropped.</summary>
        Landed,
    }
}
