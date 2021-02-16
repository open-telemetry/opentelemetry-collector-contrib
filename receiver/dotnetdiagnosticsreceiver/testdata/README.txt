for i in {0..17}; do echo $i; hexdump -C msg.$i.bin | head; done

File contents:
0.    IPC
1.    MetadataBlock
2.    StackBlock
3-14. EventBlock
15.   SPBlock
16.   StackBlock
17.   EventBlock

13 total EventBlocks
