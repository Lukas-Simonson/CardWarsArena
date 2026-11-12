// How the hand is laid out. Up to ten cards sit in one fanned, overlapping row, as they always have; past that
// the hand wraps into more rows (the cards get no smaller than a readable sliver, then the next row starts) and
// lies flat, so it grows downwards inside its box instead of spilling over the piles and buttons around it.
// The widths themselves are CSS (see .hbtn in style.css); this says when the hand counts as crowded.
export const FAN_MAX = 10;

/** Does a hand of n cards wrap into rows (and lose its fan)? */
export const isCrowded = (n) => n > FAN_MAX;

/** The turn and drop of card k of n in a fanned hand: the middle card is straight, the ends tip outwards. */
export function fanPose(k, n) {
  const off = k - (n - 1) / 2;
  return { rotate: off * 3, drop: Math.abs(off) * 3 };
}
