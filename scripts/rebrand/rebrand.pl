#!/usr/bin/env perl
# rebrand.pl - apply map.tsv to files in one pass.
#   rebrand.pl --apply <file>...   rewrite files whose content changes (mtime untouched otherwise)
#   rebrand.pl --check <file>...   print each changed line as "file:line: old" / "+ new"; exit 1 if any
# Replacement column: literal text; $1..$9 stand for capture groups. Binary files (NUL byte) and
# files that are not valid UTF-8 are skipped with a warning. Exit 2 on usage or a missing file.
use strict; use warnings; use utf8;
use Encode qw(decode encode FB_CROAK);
use File::Basename qw(dirname);
binmode STDOUT, ':encoding(UTF-8)'; binmode STDERR, ':encoding(UTF-8)';

my $mode = shift // '';
my $usage = "usage: $0 --apply|--check <file>...\n";
if (grep { /^(-h|--help)$/ } $mode, @ARGV) { print $usage; exit 0 }
unless ($mode =~ /^--(apply|check)$/ && @ARGV) { print STDERR $usage; exit 2 }
my $map = dirname(__FILE__) . '/map.tsv';
open my $m, '<:encoding(UTF-8)', $map or die "$map: $!\n";
my @rules;
while (<$m>) { chomp; next if /^#/ || /^\s*$/; my ($re, $to) = split /\t/, $_, 2; push @rules, [qr/$re/, $to]; }
close $m;

# keep.tsv: "<path glob>\t<regex>" rows. Matches in a file the glob selects are shielded from every rule
# (names a third party defines, which the map would otherwise rewrite). Globs are relative to the repo root.
my @keep;
my $keepf = dirname(__FILE__) . '/keep.tsv';
if (open my $k, '<:encoding(UTF-8)', $keepf) {
  while (<$k>) { chomp; next if /^#/ || /^\s*$/; my ($g, $re) = split /\t/, $_, 2;
    (my $gre = quotemeta $g) =~ s{\\\*\\\*}{.*}g; $gre =~ s{\\\*}{[^/]*}g; push @keep, [qr/^$gre$/, qr/$re/] }
}
my $root = `git rev-parse --show-toplevel 2>/dev/null`; chomp $root;

sub rewrite {
  my ($s, @shield) = @_;
  my @held;
  for my $re (@shield) { $s =~ s/$re/push @held, $&; "\x{E000}" . $#held . "\x{E001}"/ge }
  for my $r (@rules) {
    my ($re, $to) = @$r;
    $s =~ s/$re/my @g = ($1, $2, $3, $4, $5, $6, $7, $8, $9); (my $t = $to) =~ s{\$([1-9])(?!\d)}{$g[$1 - 1] \/\/ ""}ge; $t/ge;
  }
  $s =~ s/\x{E000}(\d+)\x{E001}/$held[$1]/g;
  return $s;
}

my $changed = 0;
for my $f (@ARGV) {
  -f $f or do { print STDERR "$f: not a file\n"; exit 2 };
  open my $in, '<:raw', $f or do { print STDERR "$f: $!\n"; exit 2 };
  local $/; my $bytes = <$in>; close $in;
  if (index($bytes, "\0") >= 0) { print STDERR "$f: binary, skipped\n"; next }
  my $text = eval { decode('UTF-8', $bytes, FB_CROAK) };
  if (!defined $text) { print STDERR "$f: not valid UTF-8, skipped\n"; next }
  require Cwd; my $abs = Cwd::abs_path($f) // $f; (my $rel = $abs) =~ s{^\Q$root\E/}{};
  my $out = rewrite($text, map { $_->[1] } grep { $rel =~ $_->[0] } @keep);
  next if $out eq $text;
  $changed++;
  if ($mode eq '--apply') {
    open my $o, '>:raw', $f or die "$f: $!\n"; print {$o} encode('UTF-8', $out); close $o;
  } else {
    my @a = split /\n/, $text, -1; my @b = split /\n/, $out, -1;
    for my $i (0 .. $#a) {
      next if defined $b[$i] && $a[$i] eq $b[$i];
      printf "%s:%d: %s\n+ %s\n", $f, $i + 1, $a[$i], $b[$i] // '';
    }
  }
}
exit($mode eq '--check' && $changed ? 1 : 0);
